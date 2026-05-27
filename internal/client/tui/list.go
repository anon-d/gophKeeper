package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	grpcClient "github.com/anon-d/gophKeeper/internal/client/grpc"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

type listModel struct {
	client  *grpcClient.Client
	secrets []*pb.Secret
	cursor  int
	err     string
}

// Сообщения
type secretsLoadedMsg struct{ secrets []*pb.Secret }
type secretsErrMsg struct{ err error }
type secretDeletedMsg struct{}
type viewSecretMsg struct{ id string }

func newListModel(client *grpcClient.Client) listModel {
	return listModel{client: client}
}

func (m listModel) loadSecrets() tea.Cmd {
	return func() tea.Msg {
		secrets, err := m.client.ListSecrets(context.Background())
		if err != nil {
			return secretsErrMsg{err}
		}
		return secretsLoadedMsg{secrets}
	}
}

func (m listModel) Init() tea.Cmd { return m.loadSecrets() }

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.secrets)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.secrets) > 0 {
				return m, func() tea.Msg {
					return viewSecretMsg{id: m.secrets[m.cursor].GetId()}
				}
			}
		case "n":
			return m, func() tea.Msg { return switchToCreateMsg{} }
		case "d":
			if len(m.secrets) > 0 {
				return m, m.deleteSecret(m.secrets[m.cursor].GetId())
			}
		case "r":
			return m, m.loadSecrets()
		}

	case secretsLoadedMsg:
		m.secrets = msg.secrets
		m.cursor = 0
		m.err = ""
	case secretsErrMsg:
		m.err = msg.err.Error()
	case secretDeletedMsg:
		return m, m.loadSecrets()
	}

	return m, nil
}

func (m listModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("📋 Секреты") + "\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render("✗ "+m.err) + "\n\n")
	}

	if len(m.secrets) == 0 {
		b.WriteString(subtleStyle.Render("  Пусто. Нажмите n чтобы создать.") + "\n")
	}

	for i, s := range m.secrets {
		cursor := "  "
		style := subtleStyle
		if i == m.cursor {
			cursor = "▸ "
			style = selectedStyle
		}
		typeName := secretTypeName(s.GetType())
		b.WriteString(style.Render(fmt.Sprintf("%s[%s] %s", cursor, typeName, s.GetTitle())) + "\n")
	}

	b.WriteString(helpStyle.Render("\n↑/↓ — навигация • enter — открыть • n — создать • d — удалить • r — обновить • q — выход"))

	return b.String()
}

func (m listModel) deleteSecret(id string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.DeleteSecret(context.Background(), id); err != nil {
			return secretsErrMsg{err}
		}
		return secretDeletedMsg{}
	}
}

func secretTypeName(t pb.SecretType) string {
	switch t {
	case pb.SecretType_CREDENTIALS:
		return "CRED"
	case pb.SecretType_PASSWORD:
		return "PASS"
	case pb.SecretType_BANKCARD:
		return "CARD"
	case pb.SecretType_TEXT:
		return "TEXT"
	case pb.SecretType_BINARY:
		return "FILE"
	default:
		return "????"
	}
}
