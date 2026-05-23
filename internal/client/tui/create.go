package tui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/anon-d/gophKeeper/internal/client/crypto"
	grpcClient "github.com/anon-d/gophKeeper/internal/client/grpc"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

type createStep int

const (
	stepType    createStep = iota // Выбор типа
	stepFields                   // Заполнение полей
)

type createModel struct {
	client     *grpcClient.Client
	vault      *crypto.Vault
	step       createStep
	typeIdx    int
	types      []pb.SecretType
	typeNames  []string
	inputs     []textinput.Model
	labels     []string
	focused    int
	err        string
	titleInput textinput.Model
	metaInput  textinput.Model
}

type switchToCreateMsg struct{}
type secretCreatedMsg struct{}
type uploadProgressMsg struct{ percent float64 }
type uploadCompleteMsg struct{}
type uploadErrMsg struct{ err error }

func newCreateModel(client *grpcClient.Client, vault *crypto.Vault) createModel {
	title := textinput.New()
	title.Placeholder = "название секрета"

	meta := textinput.New()
	meta.Placeholder = "метаинформация (необязательно)"

	return createModel{
		client:    client,
		vault:     vault,
		types:     []pb.SecretType{pb.SecretType_CREDENTIALS, pb.SecretType_PASSWORD, pb.SecretType_BANKCARD, pb.SecretType_TEXT, pb.SecretType_BINARY},
		typeNames: []string{"Учётные данные", "Пароль", "Банковская карта", "Текстовая заметка", "Файл (бинарные данные)"},
		titleInput: title,
		metaInput:  meta,
	}
}

func (m createModel) Init() tea.Cmd { return nil }

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.step == stepFields {
				m.step = stepType
				m.err = ""
				return m, nil
			}
			return m, func() tea.Msg { return switchToListMsg{} }
		}

		if m.step == stepType {
			return m.updateTypeStep(msg)
		}
		// Навигация по полям (enter/up/down)
		if m.step == stepFields {
			switch msg.String() {
			case "enter", "up", "down", "shift+tab":
				return m.updateFieldsStep(msg)
			}
		}

	case secretCreatedMsg:
		return m, func() tea.Msg { return switchToListMsg{} }
	case secretsErrMsg:
		m.err = msg.err.Error()
	}

	// Символьный ввод в текущее поле
	if m.step == stepFields {
		return m.updateFieldInputs(msg)
	}
	return m, nil
}

func (m createModel) updateTypeStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.typeIdx > 0 {
			m.typeIdx--
		}
	case "down", "j":
		if m.typeIdx < len(m.types)-1 {
			m.typeIdx++
		}
	case "enter":
		m.step = stepFields
		m.initFields()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *createModel) initFields() {
	m.titleInput.Focus()
	m.focused = -1 // -1 = title, -2 = meta, 0+ = поля

	switch m.types[m.typeIdx] {
	case pb.SecretType_CREDENTIALS:
		m.labels = []string{"Логин", "Пароль", "URL"}
		m.inputs = makeInputs("логин", "пароль", "https://...")
	case pb.SecretType_PASSWORD:
		m.labels = []string{"Пароль"}
		m.inputs = makeInputs("пароль")
	case pb.SecretType_BANKCARD:
		m.labels = []string{"Номер", "Владелец", "Срок (MM/YY)", "CVV"}
		m.inputs = makeInputs("1234 5678 9012 3456", "IVAN IVANOV", "12/25", "123")
	case pb.SecretType_TEXT:
		m.labels = []string{"Текст"}
		m.inputs = makeInputs("текст заметки")
	case pb.SecretType_BINARY:
		m.labels = []string{"Путь к файлу"}
		m.inputs = makeInputs("/path/to/file")
	}
}

func makeInputs(placeholders ...string) []textinput.Model {
	inputs := make([]textinput.Model, len(placeholders))
	for i, p := range placeholders {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = p
	}
	return inputs
}

func (m createModel) updateFieldsStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalFields := len(m.inputs) + 2 // title + meta + payload fields

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

func (m *createModel) blurCurrent() {
	switch {
	case m.focused == -1:
		m.titleInput.Blur()
	case m.focused == 0:
		m.metaInput.Blur()
	default:
		m.inputs[m.focused-1].Blur()
	}
}

func (m *createModel) focusCurrent() {
	switch {
	case m.focused == -1:
		m.titleInput.Focus()
	case m.focused == 0:
		m.metaInput.Focus()
	default:
		m.inputs[m.focused-1].Focus()
	}
}

func (m createModel) updateFieldInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m createModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("➕ Новый секрет") + "\n\n")

	if m.step == stepType {
		b.WriteString(labelStyle.Render("Выберите тип:") + "\n\n")
		for i, name := range m.typeNames {
			cursor := "  "
			style := subtleStyle
			if i == m.typeIdx {
				cursor = "▸ "
				style = selectedStyle
			}
			b.WriteString(style.Render(cursor+name) + "\n")
		}
		b.WriteString(helpStyle.Render("\n↑/↓ — выбор • enter — далее • esc — назад"))
		return b.String()
	}

	// Шаг заполнения полей
	b.WriteString(labelStyle.Render("Тип: ") + m.typeNames[m.typeIdx] + "\n\n")
	b.WriteString(labelStyle.Render("Название: ") + m.titleInput.View() + "\n")
	b.WriteString(labelStyle.Render("Мета:     ") + m.metaInput.View() + "\n\n")

	for i, input := range m.inputs {
		b.WriteString(labelStyle.Render(m.labels[i]+": ") + input.View() + "\n")
	}

	if m.err != "" {
		b.WriteString("\n" + errorStyle.Render("✗ "+m.err))
	}

	b.WriteString(helpStyle.Render("\nenter — далее/сохранить • esc — назад к типу"))

	return b.String()
}

func (m createModel) submit() tea.Cmd {
	title := m.titleInput.Value()
	if title == "" {
		return func() tea.Msg { return secretsErrMsg{fmt.Errorf("название обязательно")} }
	}

	masterPass, ok := m.vault.Get()
	if !ok {
		return func() tea.Msg { return masterExpiredMsg{} }
	}

	secretType := m.types[m.typeIdx]

	// Валидация номера карты по алгоритму Луна
	if secretType == pb.SecretType_BANKCARD {
		cardNumber := getOrEmpty(m.inputValues(), 0)
		if !crypto.ValidateLuhn(cardNumber) {
			return func() tea.Msg { return secretsErrMsg{fmt.Errorf("неверный номер карты (проверка Луна)")} }
		}
	}

	// BINARY — фоновая загрузка файла через стрим
	if secretType == pb.SecretType_BINARY {
		return m.submitBinaryFile(title, masterPass)
	}

	payload := m.buildPayload()
	metaStr := m.metaInput.Value()
	vault := m.vault
	client := m.client

	return func() tea.Msg {
		encPayload, err := crypto.EncryptWithPassword(masterPass, payload)
		if err != nil {
			return secretsErrMsg{fmt.Errorf("шифрование payload: %w", err)}
		}

		encMeta := ""
		if metaStr != "" {
			encMetaBytes, err := crypto.EncryptWithPassword(masterPass, []byte(metaStr))
			if err != nil {
				return secretsErrMsg{fmt.Errorf("шифрование metadata: %w", err)}
			}
			encMeta = base64.StdEncoding.EncodeToString(encMetaBytes)
		}

		vault.Refresh()

		secret := &pb.Secret{
			Id:       uuid.New().String(),
			Type:     secretType,
			Title:    title,
			Payload:  encPayload,
			Metadata: encMeta,
		}

		if err := client.CreateSecret(context.Background(), secret); err != nil {
			return secretsErrMsg{err}
		}
		return secretCreatedMsg{}
	}
}

// submitBinaryFile запускает фоновую загрузку файла с потоковым шифрованием.
func (m createModel) submitBinaryFile(title, masterPass string) tea.Cmd {
	filePath := getOrEmpty(m.inputValues(), 0)
	if filePath == "" {
		return func() tea.Msg { return secretsErrMsg{fmt.Errorf("укажите путь к файлу")} }
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return func() tea.Msg { return secretsErrMsg{fmt.Errorf("файл не найден: %w", err)} }
	}
	totalSize := info.Size()
	client := m.client
	vault := m.vault

	return func() tea.Msg {
		// Параметры потокового шифрования
		streamMeta, err := crypto.NewStreamMeta()
		if err != nil {
			return uploadErrMsg{err}
		}
		streamMeta.Filename = filepath.Base(filePath) // сохраняем оригинальное имя с расширением
		metaJSON, err := streamMeta.Encode()
		if err != nil {
			return uploadErrMsg{err}
		}

		file, err := os.Open(filePath)
		if err != nil {
			return uploadErrMsg{fmt.Errorf("открыть файл: %w", err)}
		}
		defer file.Close()

		// Оборачиваем файл в AES-CTR шифрование
		encReader, err := crypto.NewEncryptReader(masterPass, streamMeta, file)
		if err != nil {
			return uploadErrMsg{err}
		}

		secret := &pb.Secret{
			Id:       uuid.New().String(),
			Type:     pb.SecretType_BINARY,
			Title:    title,
			Metadata: metaJSON, // salt + IV для дешифровки
		}

		vault.Refresh()

		err = client.UploadFile(context.Background(), secret, encReader, totalSize, nil)
		if err != nil {
			return uploadErrMsg{err}
		}
		return uploadCompleteMsg{}
	}
}

func (m createModel) inputValues() []string {
	values := make([]string, len(m.inputs))
	for i, input := range m.inputs {
		values[i] = input.Value()
	}
	return values
}

func (m createModel) buildPayload() []byte {
	values := m.inputValues()

	switch m.types[m.typeIdx] {
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

func getOrEmpty(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}
