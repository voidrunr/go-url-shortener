# internal/config

В этом пакете хранятся конфигурации приложения.

Загрузку конфигурации можно реализовать из различных источников, например: файлов, переменных окружения, баз данных и других.

## Источники конфигурации и приоритеты

Порядок применения значений (каждый следующий перебивает предыдущий):

1. значения по умолчанию;
2. JSON-файл конфигурации (путь: флаг `-c` / `-config` или переменная окружения `CONFIG`);
3. флаги, заданные явно при запуске (`-a`, `-b`, `-m`, `-f`, `-d`, `-secret-key`, `-token-ttl`, `-audit-file`, `-audit-url`, `-pprof-address`, `-s`, `-t`);
4. переменные окружения (`SERVER_ADDRESS`, `BASE_URL`, `FILE_STORAGE_PATH`, `DATABASE_DSN`, `SECRET_KEY`, `TOKEN_TTL`, `AUDIT_FILE`, `AUDIT_URL`, `PPROF_ADDRESS`, `ENABLE_HTTPS`, `TRUSTED_SUBNET`, `CONFIG`).

Применяются только ключи, присутствующие в файле; отсутствующие ключи не перезаписывают дефолты.

## Формат файла конфигурации

Строгий JSON (без комментариев). Поддерживаются все опции приложения:

```json
{
    "server_address": "localhost:8080",
    "base_url": "http://localhost",
    "collision_retries": 5,
    "file_storage_path": "/path/to/file.db",
    "database_dsn": "",
    "secret_key": "ChangeThisStringImportant",
    "token_ttl": "24h",
    "audit_file": "",
    "audit_url": "",
    "pprof_address": "",
    "enable_https": true,
    "trusted_subnet": ""
}
```

При `enable_https: true` (или флаге `-s`) схема `http://` в `base_url` заменяется на `https://`.
