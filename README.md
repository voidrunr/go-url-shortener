# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

### Бенчмарки

```bash
go test -run='^$' -bench=. -benchmem ./internal/service ./internal/repository
```

**до**:

```
BenchmarkShorten-12                   15198    135765 ns/op    1144 B/op    10 allocs/op
BenchmarkShortenBatch-12              31867     33875 ns/op    2467 B/op    20 allocs/op
BenchmarkResolve-12                32459680        32.99 ns/op    0 B/op     0 allocs/op
BenchmarkMemoryRepositoryWrite-12     17654    109492 ns/op     624 B/op     3 allocs/op
BenchmarkMemoryRepositoryGet-12    59398219        19.30 ns/op    0 B/op     0 allocs/op
BenchmarkFileRepositoryWrite-12        3495  15316202 ns/op 2245093 B/op  3519 allocs/op
BenchmarkFileRepositoryWriteBatch-12   1198  13716231 ns/op 1355333 B/op  2423 allocs/op
BenchmarkFileRepositoryRead-12          891   1328076 ns/op  114440 B/op   431 allocs/op
```

**после**:

```
BenchmarkShorten-12                  133324     13020 ns/op     883 B/op     5 allocs/op
BenchmarkShortenBatch-12              65462     22403 ns/op    1713 B/op     8 allocs/op
BenchmarkResolve-12                27976056        56.95 ns/op    0 B/op     0 allocs/op
BenchmarkMemoryRepositoryWrite-12   126453       8098 ns/op     859 B/op     4 allocs/op
BenchmarkMemoryRepositoryGet-12    41647182       36.61 ns/op    0 B/op     0 allocs/op
BenchmarkFileRepositoryWrite-12       23787      59762 ns/op    1223 B/op    10 allocs/op
BenchmarkFileRepositoryWriteBatch-12  21416      51985 ns/op    3023 B/op    21 allocs/op
BenchmarkFileRepositoryRead-12          385    2710428 ns/op   115288 B/op   930 allocs/op
```

Снимок профиля памяти:

```bash
go test -run='^$' -bench=. -benchmem -memprofilerate=1 \
    -memprofile=/tmp/base-service.pprof ./internal/service
go test -run='^$' -bench=. -benchmem -memprofilerate=1 \
    -memprofile=/tmp/base-repo.pprof ./internal/repository
# -memprofile не работает с несколькими пакетами, объединяем профили
go tool pprof -proto /tmp/base-service.pprof /tmp/base-repo.pprof > profiles/base.pprof
```

```bash
go tool pprof -top -sample_index=alloc_objects profiles/base.pprof
go tool pprof -list 'URLService.Shorten$' -sample_index=alloc_objects profiles/base.pprof
go tool pprof -peek 'time.Time.MarshalJSON' profiles/base.pprof
go tool pprof -http=:6060 profiles/base.pprof   # web
```

```
File: service.test
Type: alloc_objects
Showing nodes accounting for 16320277, 99.13% of 16463783 total
      flat  flat%   sum%        cum   cum%
  15151930 92.03% 92.03%   15151938 92.03%  time.Time.MarshalJSON
    218480  1.33% 93.36%     351547  2.14%  net/url.(*URL).joinPath
    198418  1.21% 94.56%     198418  1.21%  encoding/json.(*decodeState).literalStore
    109240  0.66% 95.89%     109240  0.66%  encoding/hex.EncodeToString (inline)
    109240  0.66% 96.55%     109240  0.66%  net/url.parse
     85422  0.52% 97.63%      85422  0.52%  encoding/base64.(*Encoding).EncodeToString
     23827  0.14% 98.80%     133067  0.81%  path.Join
```

```
File: service.test
Type: alloc_space
Showing nodes accounting for -8869553.66kB, 93.06% of 9530578.42kB total
      flat  flat%   sum%        cum   cum%
-3430936.45kB 36.00% 36.00% -8303691.60kB 87.13%  encoding/json.MarshalIndent
-2428072.25kB 25.48% 61.48% -2428072.25kB 25.48%  bytes.growSlice
-1703413.31kB 17.87% 79.35% -4838685.39kB 50.77%  encoding/json.Marshal
 -964721.70kB 10.12% 89.47% -9269265.60kB 97.26%  github.com/voidrunr/go-url-shortener/internal/repository.(*FileRepository).saveToFile
 -701132.16kB  7.36% 96.83% -701132.16kB  7.36%  time.Time.MarshalJSON
  179801.39kB  1.89% 94.94%  179801.39kB  1.89%  github.com/voidrunr/go-url-shortener/internal/repository.(*MemoryRepository).Write
   43557.98kB  0.46% 93.64% -1507464.54kB 15.82%  github.com/voidrunr/go-url-shortener/internal/repository.(*FileRepository).WriteBatch
```

По числу аллокаций (отрицательные значения -  улучшение):

```bash
go tool pprof -top -sample_index=alloc_objects -diff_base=profiles/base.pprof profiles/result.pprof
```

```
Type: alloc_objects
Showing nodes accounting for -13424706, 81.54% of 16463783 total
      flat  flat%   sum%        cum   cum%
 -14957486 90.85% 90.85%  -14957486 90.85%  time.Time.MarshalJSON
   -218480  1.33% 84.72%    -351547  2.14%  net/url.(*URL).joinPath
   -109240  0.66% 81.90%    -109240  0.66%  net/url.parse
    -85422  0.52% 82.48%     -85422  0.52%  encoding/base64.(*Encoding).EncodeToString
    -85413  0.52% 82.99%     -85413  0.52%  path.(*lazybuf).append (inline)
```

```bash
# флаг --pprof-address или переменная PPROF_ADDRESS; пусто = отключено
go run ./cmd/shortener -a localhost:8080 -f /tmp/store.json -pprof-address localhost:6060
curl -s localhost:6060/debug/pprof/heap      # heap живой памяти (без принудительного GC)
curl -s 'localhost:6060/debug/pprof/profile?seconds=10' > cpu.pprof
```

тестовые данные `testdata/urls.txt` 1000 уникальных url

сценарий нагрузки одной командой:

```bash
go install github.com/rakyll/hey@latest   # once; hey используется для soak на горячем коде
./scripts/loadtest.sh
```

**до оптимизации gzip**:

```
loadgen: 4 179 rps, p50=8.94ms  p95=34.54ms  p99=47.03ms, ошибок 0
hey:     3 384 rps, avg=14.8ms  (33 883 ответа 307)

go tool pprof -top profiles/load.pprof:
    39.99%  compress/flate.NewWriter          # состояние gzip-компрессора на каждый запрос
    22.98%  repository.(*FileRepository).Write # карта хранилища (данные)
CPU: ~50% времени — GC (spanClass/scanObject/memclr), 6% — flate.deflate
```

```bash
go test -run='^$' -bench=Gzip -benchmem ./internal/middleware
```

| Сценарий | До | После (пул + skip 3xx) |
|---|---|---|
| редирект 307 | 81–158 µs/op, **816 KB/op**, 35 allocs | 1.16 µs/op, 1.4 KB/op, 16 allocs |
| JSON через gzip | — | 23 µs/op, 1.2 KB/op, 12 allocs |

**после**:

```
loadgen: 26 762 rps (×6.4), p50=1.22ms  p95=5.59ms  p99=8.06ms, ошибок 0
hey:     37 191 rps (×11), avg=1.3ms  (372k ответов 307)

go tool pprof -top profiles/load.pprof:
    45.87%  repository.(*FileRepository).Write  # теперь это данные: 134k записей ≈ 40MB
    (compress/flate.* больше не попадает в топ)
CPU: распределён без доминанты, GC — лишь доля syscall-события
```
