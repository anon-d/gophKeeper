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
<video src="./assets/reg.mp4" width="600" controls autoplay muted loop></video>

**Создание секрета**
<video src="./assets/use.mp4" width="600" controls autoplay muted loop></video>

## Конфигурирование
Для конфигурирования переменных окружения используйте файл `.env`. Пример файла `.env` можно найти в `.env.example`.
