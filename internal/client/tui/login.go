package tui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anon-d/gophKeeper/internal/client/crypto"
	grpcClient "github.com/anon-d/gophKeeper/internal/client/grpc"
)

type loginModel struct {
	client   *grpcClient.Client
	vault    *crypto.Vault
	inputs   []textinput.Model
	focused  int
	err      string
	register bool // true = режим регистрации
}

func newLoginModel(client *grpcClient.Client, vault *crypto.Vault) loginModel {
	username := textinput.New()
	username.Placeholder = "username"
	username.Focus()

	password := textinput.New()
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'

	masterPass := textinput.New()
	masterPass.Placeholder = "мастер-пароль"
	masterPass.EchoMode = textinput.EchoPassword
	masterPass.EchoCharacter = '•'

	return loginModel{
		client: client,
		vault:  vault,
		inputs: []textinput.Model{username, password, masterPass},
	}
}

// Сообщения
type loginSuccessMsg struct{}
type loginErrMsg struct{ err error }

func (m loginModel) Init() tea.Cmd { return textinput.Blink }

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.register = !m.register
			m.err = ""
			return m, nil
		case "enter":
			if m.focused < len(m.inputs)-1 {
				m.inputs[m.focused].Blur()
				m.focused++
				m.inputs[m.focused].Focus()
				return m, textinput.Blink
			}
			return m, m.submit()
		case "shift+tab", "up":
			if m.focused > 0 {
				m.inputs[m.focused].Blur()
				m.focused--
				m.inputs[m.focused].Focus()
			}
			return m, nil
		case "down":
			if m.focused < len(m.inputs)-1 {
				m.inputs[m.focused].Blur()
				m.focused++
				m.inputs[m.focused].Focus()
			}
			return m, nil
		}

	case loginSuccessMsg:
		return m, nil // app.go перехватит
	case loginErrMsg:
		m.err = msg.err.Error()
		return m, nil
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m loginModel) View() string {
	var b strings.Builder

	mode := "Вход"
	if m.register {
		mode = "Регистрация"
	}
	b.WriteString(titleStyle.Render("🔐 GophKeeper — "+mode) + "\n\n")

	labels := []string{"Логин:         ", "Пароль:        ", "Мастер-пароль: "}
	for i, input := range m.inputs {
		b.WriteString(labelStyle.Render(labels[i]) + input.View() + "\n")
	}

	if m.err != "" {
		b.WriteString("\n" + errorStyle.Render("✗ "+m.err))
	}

	b.WriteString(helpStyle.Render("\ntab — переключить вход/регистрация • enter — отправить • esc — выход"))

	return b.String()
}

func (m loginModel) submit() tea.Cmd {
	username := m.inputs[0].Value()
	password := m.inputs[1].Value()
	masterPass := m.inputs[2].Value()
	if username == "" || password == "" || masterPass == "" {
		return func() tea.Msg {
			return loginErrMsg{fmt.Errorf("заполните все поля")}
		}
	}

	passHash := hashPassword(password)
	vault := m.vault

	return func() tea.Msg {
		ctx := context.Background()
		if m.register {
			if err := m.client.Register(ctx, username, passHash); err != nil {
				return loginErrMsg{err}
			}
		}
		if err := m.client.Login(ctx, username, passHash); err != nil {
			return loginErrMsg{err}
		}
		// Сохраняем мастер-пароль в vault на 15 минут
		vault.Store(masterPass)
		return loginSuccessMsg{}
	}
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", h)
}
