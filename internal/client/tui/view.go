package tui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anon-d/gophKeeper/internal/client/crypto"
	grpcClient "github.com/anon-d/gophKeeper/internal/client/grpc"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

type viewModel struct {
	client           *grpcClient.Client
	vault            *crypto.Vault
	secret           *pb.Secret
	decryptedPayload []byte
	decryptedMeta    string
	err              string
	downloading      bool
	downloadPath     string
}

type downloadCompleteMsg struct{ path string }
type downloadErrMsg struct{ err error }

type secretLoadedMsg struct{ secret *pb.Secret }
type switchToListMsg struct{}
type masterExpiredMsg struct{}

func newViewModel(client *grpcClient.Client, vault *crypto.Vault, secretID string) viewModel {
	return viewModel{client: client, vault: vault}
}

func loadSecret(client *grpcClient.Client, vault *crypto.Vault, id string) tea.Cmd {
	return func() tea.Msg {
		secret, err := client.GetSecret(context.Background(), id)
		if err != nil {
			return secretsErrMsg{err}
		}
		return secretLoadedMsg{secret}
	}
}

func (m viewModel) Init() tea.Cmd { return nil }

func (m viewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", "backspace":
			return m, func() tea.Msg { return switchToListMsg{} }
		case "s":
			if m.secret != nil && m.secret.GetType() == pb.SecretType_BINARY && !m.downloading {
				m.downloading = true
				return m, m.startDownload()
			}
		case "e":
			if m.secret != nil && m.secret.GetType() != pb.SecretType_BINARY {
				return m, func() tea.Msg {
					return switchToEditMsg{
						secret:  m.secret,
						payload: m.decryptedPayload,
						meta:    m.decryptedMeta,
					}
				}
			}
		}
	case secretLoadedMsg:
		m.secret = msg.secret
		m.err = ""
		// Дешифровка при получении
		masterPass, ok := m.vault.Get()
		if !ok {
			return m, func() tea.Msg { return masterExpiredMsg{} }
		}
		if len(msg.secret.GetPayload()) > 0 {
			dec, err := crypto.DecryptWithPassword(masterPass, msg.secret.GetPayload())
			if err != nil {
				m.err = "ошибка дешифрования: " + err.Error()
			} else {
				m.decryptedPayload = dec
			}
		}
		if meta := msg.secret.GetMetadata(); meta != "" {
			metaBytes, err := base64.StdEncoding.DecodeString(meta)
			if err == nil {
				dec, err := crypto.DecryptWithPassword(masterPass, metaBytes)
				if err == nil {
					m.decryptedMeta = string(dec)
				}
			}
		}
		m.vault.Refresh()
	case downloadCompleteMsg:
		m.downloading = false
		m.downloadPath = msg.path
	case downloadErrMsg:
		m.downloading = false
		m.err = msg.err.Error()
	case secretsErrMsg:
		m.err = msg.err.Error()
	}
	return m, nil
}

func (m viewModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔍 Просмотр секрета") + "\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render("✗ "+m.err) + "\n")
		b.WriteString(helpStyle.Render("\nesc — назад"))
		return b.String()
	}

	if m.secret == nil {
		b.WriteString(subtleStyle.Render("Загрузка...") + "\n")
		return b.String()
	}

	s := m.secret
	b.WriteString(labelStyle.Render("Тип:      ") + secretTypeName(s.GetType()) + "\n")
	b.WriteString(labelStyle.Render("Название: ") + s.GetTitle() + "\n")
	b.WriteString(labelStyle.Render("ID:       ") + s.GetId() + "\n")
	b.WriteString(labelStyle.Render("Версия:   ") + fmt.Sprintf("%d", s.GetVersion()) + "\n")

	if m.decryptedMeta != "" {
		b.WriteString(labelStyle.Render("Мета:     ") + m.decryptedMeta + "\n")
	}

	if s.GetType() == pb.SecretType_BINARY {
		b.WriteString("\n" + subtleStyle.Render("[бинарные данные в MinIO]") + "\n")
		if m.downloading {
			b.WriteString(successStyle.Render("↓ Скачивание...") + "\n")
		} else if m.downloadPath != "" {
			b.WriteString(successStyle.Render(fmt.Sprintf("✓ Сохранено: %s", m.downloadPath)) + "\n")
		}
	} else if len(m.decryptedPayload) > 0 {
		b.WriteString("\n" + labelStyle.Render("Данные:") + "\n")
		b.WriteString(renderPayload(s.GetType(), m.decryptedPayload))
	}

	help := "esc — назад • q — выход"
	if s.GetType() == pb.SecretType_BINARY && !m.downloading {
		help = "s — скачать файл • " + help
	} else if s.GetType() != pb.SecretType_BINARY {
		help = "e — редактировать • " + help
	}
	b.WriteString(helpStyle.Render("\n" + help))

	return b.String()
}

// startDownload запускает фоновое скачивание файла с дешифрованием.
func (m viewModel) startDownload() tea.Cmd {
	secret := m.secret
	client := m.client
	vault := m.vault

	return func() tea.Msg {
		masterPass, ok := vault.Get()
		if !ok {
			return masterExpiredMsg{}
		}

		// Путь для сохранения: ~/Downloads/<оригинальное имя файла>
		streamMeta, err := crypto.DecodeStreamMeta(secret.GetMetadata())
		if err != nil {
			return downloadErrMsg{fmt.Errorf("метаданные шифрования: %w", err)}
		}

		filename := streamMeta.Filename
		if filename == "" {
			filename = secret.GetTitle() // fallback
		}
		home, _ := os.UserHomeDir()
		outPath := filepath.Join(home, "Downloads", filename)

		outFile, err := os.Create(outPath)
		if err != nil {
			return downloadErrMsg{fmt.Errorf("создать файл: %w", err)}
		}
		defer outFile.Close()

		// Pipe: gRPC stream → pipe → decryptReader → outFile
		pr, pw := io.Pipe()

		// Дешифрующий reader
		decReader, err := crypto.NewDecryptReader(masterPass, streamMeta, pr)
		if err != nil {
			pr.Close()
			return downloadErrMsg{err}
		}

		// Фоновое скачивание и запись в pipe
		errCh := make(chan error, 1)
		go func() {
			defer pw.Close()
			_, err := client.DownloadFile(context.Background(), secret.GetId(), pw, nil)
			if err != nil {
				pw.CloseWithError(err)
			}
			errCh <- err
		}()

		// Читаем дешифрованные данные и пишем в файл
		if _, err := io.Copy(outFile, decReader); err != nil {
			return downloadErrMsg{fmt.Errorf("запись файла: %w", err)}
		}

		if err := <-errCh; err != nil {
			return downloadErrMsg{err}
		}

		vault.Refresh()
		return downloadCompleteMsg{path: outPath}
	}
}

// renderPayload пытается отобразить payload в зависимости от типа.
func renderPayload(t pb.SecretType, data []byte) string {
	switch t {
	case pb.SecretType_CREDENTIALS:
		var creds struct {
			Login    string `json:"login"`
			Password string `json:"password"`
			URL      string `json:"url,omitempty"`
		}
		if json.Unmarshal(data, &creds) == nil {
			var b strings.Builder
			b.WriteString("  Логин:  " + creds.Login + "\n")
			b.WriteString("  Пароль: " + creds.Password + "\n")
			if creds.URL != "" {
				b.WriteString("  URL:    " + creds.URL + "\n")
			}
			return b.String()
		}
	case pb.SecretType_BANKCARD:
		var card struct {
			Number string `json:"number"`
			Holder string `json:"holder"`
			Expiry string `json:"expiry"`
			CVV    string `json:"cvv"`
		}
		if json.Unmarshal(data, &card) == nil {
			var b strings.Builder
			b.WriteString("  Номер:    " + card.Number + "\n")
			b.WriteString("  Владелец: " + card.Holder + "\n")
			b.WriteString("  Срок:     " + card.Expiry + "\n")
			b.WriteString("  CVV:      " + card.CVV + "\n")
			return b.String()
		}
	case pb.SecretType_TEXT, pb.SecretType_PASSWORD:
		return "  " + string(data) + "\n"
	case pb.SecretType_BINARY:
		return fmt.Sprintf("  [бинарные данные, %d байт]\n", len(data))
	}
	return "  " + string(data) + "\n"
}
