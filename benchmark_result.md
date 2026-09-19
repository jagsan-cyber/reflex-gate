# JEV benchmark result

- endpoint: `http://127.0.0.1:8090`
- model: `Qwen3.5-0.8B-Q8_0` (local llama-server)
- enable_thinking: `False`
- n: 73

## 1. TTFT by prompt length (prefill)

| prompt | n | mean prompt tok | TTFT s | prefill tok/s |
| --- | --- | --- | --- | --- |
| 1K | 3 | 0 | 0.000 +/- 0.000 | 0.0 |
| 2K | 3 | 0 | 0.000 +/- 0.000 | 0.0 |
| 4K | 2 | 0 | 0.000 +/- 0.000 | 0.0 |
| 8K | 2 | 0 | 0.000 +/- 0.000 | 0.0 |

## 2. Decode speed (short generation)

| setting | out tok | TTFT s | decode s | total s | tok/s |
| --- | --- | --- | --- | --- | --- |
| 5 tok target | 5 | 0.144 | 0.061 | 0.205 | 82.16 |
| 30 tok target | 30 | 0.132 | 0.438 | 0.570 | 68.49 |
| 100 tok target | 21 | 0.126 | 0.301 | 0.427 | 69.72 |

## 3. Accuracy

| task | name | n | accuracy |
| --- | --- | --- | --- |
| A | Yes/No stop | 30 | 100.0% (30/30) |
| B | JSON extract | 30 | 100.0% (30/30) |
| C | needle line | 10 | 100.0% (10/10) |

## 4. Format compliance

| task | name | n | compliance |
| --- | --- | --- | --- |
| A | Yes/No word only | 30 | 100.0% (30/30) |
| B | JSON schema | 30 | 100.0% (30/30) |
| C | line only, no preamble | 10 | 100.0% (10/10) |

## Failures

None.
