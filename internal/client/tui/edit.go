package tui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anon-d/gophKeeper/internal/client/crypto"
	grpcClient "github.com/anon-d/gophKeeper/internal/client/grpc"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

type editModel struct {
	client     *grpcClient.Client
	vault      *crypto.Vault
	secret     *pb.Secret // оригинальный секрет (с version для оптимистичной блокировки)
	inputs     []textinput.Model
	labels     []string
	focused    int
	err        string
	titleInput textinput.Model
	metaInput  textinput.Model
}

type switchToEditMsg struct {
	secret  *pb.Secret
	payload []byte
	meta    string
}
type secretUpdatedMsg struct{ newVersion int64 }

func newEditModel(client *grpcClient.Client, vault *crypto.Vault, secret *pb.Secret, decryptedPayload []byte, decryptedMeta string) editModel {
	title := textinput.New()
	title.Placeholder = "название"
	title.SetValue(secret.GetTitle())
	title.Focus()

	meta := textinput.New()
	meta.Placeholder = "метаинформация"
	meta.SetValue(decryptedMeta)

	m := editModel{
		client:     client,
		vault:      vault,
		secret:     secret,
		titleInput: title,
		metaInput:  meta,
		focused:    -1,
	}

	// Заполняем поля из расшифрованного payload
	switch secret.GetType() {
	case pb.SecretType_CREDENTIALS:
		m.labels = []string{"Логин", "Пароль", "URL"}
		m.inputs = makeInputs("", "", "")
		var creds struct {
			Login    string `json:"login"`
			Password string `json:"password"`
			URL      string `json:"url"`
		}
		if json.Unmarshal(decryptedPayload, &creds) == nil {
			m.inputs[0].SetValue(creds.Login)
			m.inputs[1].SetValue(creds.Password)
			m.inputs[2].SetValue(creds.URL)
		}
	case pb.SecretType_PASSWORD:
		m.labels = []string{"Пароль"}
		m.inputs = makeInputs("")
		m.inputs[0].SetValue(string(decryptedPayload))
	case pb.SecretType_BANKCARD:
		m.labels = []string{"Номер", "Владелец", "Срок (MM/YY)", "CVV"}
		m.inputs = makeInputs("", "", "", "")
		var card struct {
			Number string `json:"number"`
			Holder string `json:"holder"`
			Expiry string `json:"expiry"`
			CVV    string `json:"cvv"`
		}
		if json.Unmarshal(decryptedPayload, &card) == nil {
			m.inputs[0].SetValue(card.Number)
			m.inputs[1].SetValue(card.Holder)
			m.inputs[2].SetValue(card.Expiry)
			m.inputs[3].SetValue(card.CVV)
		}
	case pb.SecretType_TEXT:
		m.labels = []string{"Текст"}
		m.inputs = makeInputs("")
		m.inputs[0].SetValue(string(decryptedPayload))
	}

	return m
}

func (m editModel) Init() tea.Cmd { return textinput.Blink }

func (m editModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, func() tea.Msg { return switchToListMsg{} }
		case "enter", "up", "down", "shift+tab":
			return m.updateNav(msg)
		}
	case secretUpdatedMsg:
		return m, func() tea.Msg { return switchToListMsg{} }
	case secretsErrMsg:
		m.err = msg.err.Error()
		return m, nil
	}

	// Символьный ввод
	var cmd tea.Cmd
	switch {
	case m.focused == -1:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case m.focused == 0:
		m.metaInput, cmd = m.metaInput.Update(msg)
	default:
		idx := m.focused - 1
		if idx >= 0 && idx < len(m.inputs) {
			m.inputs[idx], cmd = m.inputs[idx].Update(msg)
		}
	}
	return m, cmd
}

func (m editModel) updateNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalFields := len(m.inputs) + 2

	switch msg.String() {
	case "enter":
		if m.focused < totalFields-2 {
			m.blurCurrent()
			m.focused++
			m.focusCurrent()
			return m, textinput.Blink
		}
		return m, m.submit()
	case "shift+tab", "up":
		if m.focused > -1 {
			m.blurCurrent()
			m.focused--
			m.focusCurrent()
			return m, textinput.Blink
		}
	case "down":
		if m.focused < totalFields-2 {
			m.blurCurrent()
			m.focused++
			m.focusCurrent()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *editModel) blurCurrent() {
	switch {
	case m.focused == -1:
		m.titleInput.Blur()
	case m.focused == 0:
		m.metaInput.Blur()
	default:
		m.inputs[m.focused-1].Blur()
	}
}

func (m *editModel) focusCurrent() {
	switch {
	case m.focused == -1:
		m.titleInput.Focus()
	case m.focused == 0:
		m.metaInput.Focus()
	default:
		m.inputs[m.focused-1].Focus()
	}
}

func (m editModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("✏️  Редактирование") + "\n\n")
	b.WriteString(labelStyle.Render("Тип: ") + secretTypeName(m.secret.GetType()) + "\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("Версия: %d", m.secret.GetVersion())) + "\n\n")

	b.WriteString(labelStyle.Render("Название: ") + m.titleInput.View() + "\n")
	b.WriteString(labelStyle.Render("Мета:     ") + m.metaInput.View() + "\n\n")

	for i, input := range m.inputs {
		b.WriteString(labelStyle.Render(m.labels[i]+": ") + input.View() + "\n")
	}

	if m.err != "" {
		b.WriteString("\n" + errorStyle.Render("✗ "+m.err))
	}

	b.WriteString(helpStyle.Render("\nenter — далее/сохранить • esc — отмена"))

	return b.String()
}

func (m editModel) submit() tea.Cmd {
	title := m.titleInput.Value()
	if title == "" {
		return func() tea.Msg { return secretsErrMsg{fmt.Errorf("название обязательно")} }
	}

	masterPass, ok := m.vault.Get()
	if !ok {
		return func() tea.Msg { return masterExpiredMsg{} }
	}

	// Валидация Луна для карт
	if m.secret.GetType() == pb.SecretType_BANKCARD {
		cardNumber := m.inputs[0].Value()
		if !crypto.ValidateLuhn(cardNumber) {
			return func() tea.Msg { return secretsErrMsg{fmt.Errorf("неверный номер карты (проверка Луна)")} }
		}
	}

	payload := m.buildPayload()
	metaStr := m.metaInput.Value()
	secret := m.secret
	vault := m.vault
	client := m.client

	return func() tea.Msg {
		encPayload, err := crypto.EncryptWithPassword(masterPass, payload)
		if err != nil {
			return secretsErrMsg{fmt.Errorf("шифрование: %w", err)}
		}

		encMeta := ""
		if metaStr != "" {
			encMetaBytes, err := crypto.EncryptWithPassword(masterPass, []byte(metaStr))
			if err != nil {
				return secretsErrMsg{fmt.Errorf("шифрование мета: %w", err)}
			}
			encMeta = base64.StdEncoding.EncodeToString(encMetaBytes)
		}

		vault.Refresh()

		updated := &pb.Secret{
			Id:       secret.GetId(),
			Type:     secret.GetType(),
			Title:    title,
			Payload:  encPayload,
			Metadata: encMeta,
			Version:  secret.GetVersion(), // текущая версия для оптимистичной блокировки
		}

		newVersion, err := client.UpdateSecret(context.Background(), updated)
		if err != nil {
			return secretsErrMsg{err}
		}
		return secretUpdatedMsg{newVersion: newVersion}
	}
}

func (m editModel) buildPayload() []byte {
	values := make([]string, len(m.inputs))
	for i, input := range m.inputs {
		values[i] = input.Value()
	}

	switch m.secret.GetType() {
	case pb.SecretType_CREDENTIALS:
		data, _ := json.Marshal(map[string]string{
			"login":    getOrEmpty(values, 0),
			"password": getOrEmpty(values, 1),
			"url":      getOrEmpty(values, 2),
		})
		return data
	case pb.SecretType_BANKCARD:
		data, _ := json.Marshal(map[string]string{
			"number": getOrEmpty(values, 0),
			"holder": getOrEmpty(values, 1),
			"expiry": getOrEmpty(values, 2),
			"cvv":    getOrEmpty(values, 3),
		})
		return data
	case pb.SecretType_PASSWORD:
		return []byte(getOrEmpty(values, 0))
	case pb.SecretType_TEXT:
		return []byte(getOrEmpty(values, 0))
	}
	return nil
}
