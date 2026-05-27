// Package tui реализует терминальный пользовательский интерфейс GophKeeper.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anon-d/gophKeeper/internal/client/crypto"
	grpcClient "github.com/anon-d/gophKeeper/internal/client/grpc"
)

type screen int

const (
	screenLogin  screen = iota
	screenList
	screenView
	screenCreate
	screenEdit
	screenMasterPass // перезапрос мастер-пароля после истечения TTL
)

// App — корневая модель TUI-приложения.
type App struct {
	client     *grpcClient.Client
	vault      *crypto.Vault
	screen     screen
	prevScreen screen // куда вернуться после ввода мастер-пароля
	login      loginModel
	list       listModel
	view       viewModel
	create     createModel
	edit       editModel
	masterPass masterPassModel
}

// NewApp создаёт новое TUI-приложение.
func NewApp(client *grpcClient.Client) App {
	vault := crypto.NewVault()
	return App{
		client: client,
		vault:  vault,
		screen: screenLogin,
		login:  newLoginModel(client, vault),
	}
}

func (a App) Init() tea.Cmd {
	return a.login.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Переходы между экранами
	case loginSuccessMsg:
		a.screen = screenList
		a.list = newListModel(a.client)
		return a, a.list.Init()

	case viewSecretMsg:
		a.screen = screenView
		a.view = newViewModel(a.client, a.vault, msg.id)
		return a, loadSecret(a.client, a.vault, msg.id)

	case switchToListMsg:
		a.screen = screenList
		a.list = newListModel(a.client)
		return a, a.list.Init()

	case switchToCreateMsg:
		a.screen = screenCreate
		a.create = newCreateModel(a.client, a.vault)
		return a, a.create.Init()

	case secretCreatedMsg:
		a.screen = screenList
		a.list = newListModel(a.client)
		return a, a.list.Init()

	case switchToEditMsg:
		a.screen = screenEdit
		a.edit = newEditModel(a.client, a.vault, msg.secret, msg.payload, msg.meta)
		return a, a.edit.Init()

	case secretUpdatedMsg:
		a.screen = screenList
		a.list = newListModel(a.client)
		return a, a.list.Init()

	case uploadCompleteMsg:
		a.screen = screenList
		a.list = newListModel(a.client)
		return a, a.list.Init()

	case uploadErrMsg:
		// Прокинуть ошибку в текущий экран как secretsErrMsg
		return a.Update(secretsErrMsg{msg.err})

	case masterExpiredMsg:
		// Мастер-пароль истёк — перезапрос
		a.prevScreen = a.screen
		a.screen = screenMasterPass
		a.masterPass = newMasterPassModel(a.vault)
		return a, a.masterPass.Init()

	case masterPassEnteredMsg:
		// Вернуться на предыдущий экран
		a.screen = a.prevScreen
		return a, nil
	}

	// Делегируем текущему экрану
	var cmd tea.Cmd
	switch a.screen {
	case screenLogin:
		var m tea.Model
		m, cmd = a.login.Update(msg)
		a.login = m.(loginModel)
	case screenList:
		var m tea.Model
		m, cmd = a.list.Update(msg)
		a.list = m.(listModel)
	case screenView:
		var m tea.Model
		m, cmd = a.view.Update(msg)
		a.view = m.(viewModel)
	case screenCreate:
		var m tea.Model
		m, cmd = a.create.Update(msg)
		a.create = m.(createModel)
	case screenEdit:
		var m tea.Model
		m, cmd = a.edit.Update(msg)
		a.edit = m.(editModel)
	case screenMasterPass:
		var m tea.Model
		m, cmd = a.masterPass.Update(msg)
		a.masterPass = m.(masterPassModel)
	}
	return a, cmd
}

func (a App) View() string {
	switch a.screen {
	case screenLogin:
		return a.login.View()
	case screenList:
		return a.list.View()
	case screenView:
		return a.view.View()
	case screenCreate:
		return a.create.View()
	case screenEdit:
		return a.edit.View()
	case screenMasterPass:
		return a.masterPass.View()
	}
	return ""
}
