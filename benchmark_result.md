# JEV benchmark result

- endpoint: `http://127.0.0.1:8090`
- model: `C:\Users\fallo\.lmstudio\models\unsloth\Qwen3.5-0.8B-GGUF\Qwen3.5-0.8B-Q8_0.gguf`
- enable_thinking: `False`
- n: 73

## 1. TTFT by prompt length (prefill)

| prompt | n | mean prompt tok | TTFT s | prefill tok/s |
| --- | --- | --- | --- | --- |
| 1K | 3 | 0 | 0.000 +/- 0.000 | - |
| 2K | 3 | 0 | 0.000 +/- 0.000 | - |
| 4K | 2 | 0 | 0.000 +/- 0.000 | 0.0 |
| 8K | 2 | 0 | 0.000 +/- 0.000 | 0.0 |

## 2. Decode speed (short generation)

| setting | out tok | TTFT s | decode s | total s | tok/s |
| --- | --- | --- | --- | --- | --- |
| 5 tok target | 5 | 0.060 | 0.064 | 0.125 | 78.71 |
| 30 tok target | 30 | 0.061 | 0.445 | 0.518 | 67.46 |
| 100 tok target | 150 | 0.061 | 2.273 | 2.352 | 65.99 |

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
