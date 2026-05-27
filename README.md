# GophKeeper
![version](https://img.shields.io/badge/version-0.1.0-blue)
![license](https://img.shields.io/badge/license-MIT-green)

Простой менеджер секретов.

## Оглавление
- [Установка](#установка)
- [Использование](#использование)

## Установка

**Требования:**
- Docker + docker-cmpose;
- Task (для сборки и запуска контейнеров). Можно использовать `go tool task`.
- Go (для самостоятельной сборки клиента)

### Сервер
```bash
# Запустить сервер
task up
# Остановить сервер
task down
# Посмотреть логи
task logs

# Посмотреть список всех доступных задач:
task --list
```

### Клиент
```bash
# Собрать и запустить клиент
task run-client
```

Изменить сценарии можно в `Taskfile.yml`.

## Использование
**Регистрация/авторизация**
<video src="https://github.com/user-attachments/assets/9d7afbe1-8273-4f87-829c-a06401515404" width="800" controls></video>

**Создание секрета**
<video src="https://github.com/user-attachments/assets/eec78ef9-7a37-4e05-bd43-17537689923b" width="800" controls></video>

## Конфигурирование
Для конфигурирования переменных окружения используйте файл `.env`. Пример файла `.env` можно найти в `.env.example`.
