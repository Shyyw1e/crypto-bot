
# Load Testing (Analyser)

## 1. Scope

Нагрузочные тесты покрывают:

- `BenchmarkDetectFact`
- `BenchmarkDetectPotentialByAsks` (depth: 10/100/1000)
- `BenchmarkDetectPotentialByBids` (depth: 10/100/1000)
- `BenchmarkServiceHandleTick` (users/depth: 100/10, 1000/10, 5000/50)

Код бенчмарков:

- `internal/analyser/usecase/detection_benchmark_test.go`
- `internal/analyser/usecase/service_benchmark_test.go`

## 2. Run Commands

Быстрый прогон:

```powershell
go test ./internal/analyser/usecase -bench . -benchmem
```

Стабильное сравнение между коммитами:

```powershell
go test ./internal/analyser/usecase -bench . -benchmem -benchtime=3s -count=3
```

Только end-to-end `HandleTick`:

```powershell
go test ./internal/analyser/usecase -bench BenchmarkServiceHandleTick -benchmem -benchtime=5s
```

## 3. Baseline Results

Дата прогона: **2026-02-18**  
Окружение: `windows/amd64`, CPU `AMD Ryzen 7 5700U`

| Benchmark | Result |
|---|---|
| `BenchmarkDetectFact` | `93.85 ns/op`, `144 B/op`, `1 allocs/op` |
| `BenchmarkDetectPotentialByAsks/depth_10` | `838.4 ns/op`, `1296 B/op`, `9 allocs/op` |
| `BenchmarkDetectPotentialByAsks/depth_100` | `9412 ns/op`, `14256 B/op`, `99 allocs/op` |
| `BenchmarkDetectPotentialByAsks/depth_1000` | `96039 ns/op`, `143858 B/op`, `999 allocs/op` |
| `BenchmarkDetectPotentialByBids/depth_10` | `773.6 ns/op`, `1296 B/op`, `9 allocs/op` |
| `BenchmarkDetectPotentialByBids/depth_100` | `8446 ns/op`, `14256 B/op`, `99 allocs/op` |
| `BenchmarkDetectPotentialByBids/depth_1000` | `81373 ns/op`, `143858 B/op`, `999 allocs/op` |
| `BenchmarkServiceHandleTick/users_100_depth_10` | `1.64 ms/op`, `428304 B/op`, `14550 allocs/op` |
| `BenchmarkServiceHandleTick/users_1000_depth_10` | `16.34 ms/op`, `4211211 B/op`, `144159 allocs/op` |
| `BenchmarkServiceHandleTick/users_5000_depth_50` | `84.60 ms/op`, `21027936 B/op`, `720211 allocs/op` |

## 4. Conclusions

1. `DetectFact` работает очень быстро и почти без аллокаций.
2. `DetectPotentialByAsks/Bids` масштабируются линейно по глубине, но создают много аллокаций (почти `depth-1` на вызов).
3. `HandleTick` также масштабируется почти линейно по количеству пользователей, и основной риск под нагрузкой — рост `allocs/op` и давление на GC.
4. Текущее узкое место: детекторы potential + fan-out по активным пользователям в `HandleTick`.

## 5. Optimization Priorities

1. Убрать создание `candidate` на каждой итерации в `DetectPotentialByAsks/Bids`.
2. Снизить аллокации в `HandleTick` (буферы/переиспользование структур, уменьшение временных объектов).
3. После оптимизаций зафиксировать новую baseline-таблицу в этом же документе.

## 6. Profiling

```powershell
go test ./internal/analyser/usecase -bench BenchmarkServiceHandleTick -benchmem -cpuprofile cpu.out -memprofile mem.out
```

```powershell
go tool pprof cpu.out
go tool pprof mem.out
```


### Что хорошо

- Базовая проверка “фактического” арбитража работает очень быстро: меньше `0.0001` секунды на одну проверку.
- Даже при росте глубины стакана логика остается предсказуемой: время растет плавно, без резких скачков.
- На небольших и средних объемах система работает стабильно.

### Что сейчас ограничивает масштаб

- Самая тяжелая часть — “потенциальные” сценарии (по глубине стакана): чем глубже смотрим, тем больше создается временных объектов в памяти.
- В `HandleTick` нагрузка растет почти линейно от количества активных пользователей.
- На профиле `5000 пользователей + глубина 50` один тик занимает около `85 мс` и создает много аллокаций (`~21 MB` на операцию), что повышает давление на GC.

### Как это интерпретировать для продукта

- Сейчас система подходит для текущего этапа и умеренного роста.
- При дальнейшем росте аудитории (тысячи активных пользователей одновременно) появится риск увеличения задержек уведомлений.
- Узкое место уже найдено: оптимизация должна быть в блоке потенциальных сигналов и fan-out по пользователям.

fan-out по пользователям — это момент, когда один найденный сигнал нужно “размножить” на много пользователей и для каждого выполнить действия.

В HandleTick это выглядит так:

Нашли список возможностей (opps).
Взяли список активных пользователей (users).
Для каждой пары user x opp:
- проверить настройки (MatchUserSettings),
- проверить дедуп,
- создать уведомление в БД,
- отправить в notifier.

Почему это узкое место:

Стоимость растет примерно как кол-во пользователей * кол-во opportunities.
Даже если математика детекта быстрая, fan-out добавляет много повторяющейся работы.
На большом количестве пользователей именно эта часть начинает “съедать” время тика и память.
Простой пример:

5 opportunities за тик,
5000 активных пользователей,
это до 25 000 проверок и потенциальных действий за один тик.

### План улучшений

1. Сократить лишние аллокации в расчете potential-сигналов.
2. Оптимизировать `HandleTick` по памяти (меньше временных объектов).
3. Повторить те же тесты и зафиксировать новую baseline-метрику “до/после”.
