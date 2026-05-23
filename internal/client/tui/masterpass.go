package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anon-d/gophKeeper/internal/client/crypto"
)

type masterPassModel struct {
	vault *crypto.Vault
	input textinput.Model
	err   string
}

type masterPassEnteredMsg struct{}

func newMasterPassModel(vault *crypto.Vault) masterPassModel {
	input := textinput.New()
	input.Placeholder = "мастер-пароль"
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '•'
	input.Focus()

	return masterPassModel{
		vault: vault,
		input: input,
	}
}

func (m masterPassModel) Init() tea.Cmd { return textinput.Blink }

func (m masterPassModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, func() tea.Msg { return switchToListMsg{} }
		case "enter":
			pass := m.input.Value()
			if pass == "" {
				m.err = "введите мастер-пароль"
				return m, nil
			}
			m.vault.Store(pass)
			return m, func() tea.Msg { return masterPassEnteredMsg{} }
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m masterPassModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔑 Мастер-пароль истёк") + "\n\n")
	b.WriteString(subtleStyle.Render("Прошло 15 минут. Введите мастер-пароль повторно.") + "\n\n")
	b.WriteString(labelStyle.Render("Мастер-пароль: ") + m.input.View() + "\n")

	if m.err != "" {
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("✗ %s", m.err)))
	}

	b.WriteString(helpStyle.Render("\nenter — подтвердить • esc — к списку"))

	return b.String()
}
