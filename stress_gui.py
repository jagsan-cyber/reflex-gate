#!/usr/bin/env python3
"""local-jev High-Concurrency Indiscriminate Stress Testing GUI

A lightweight standalone dark-mode GUI for load testing local-jev.
Pure Python standard library (tkinter + urllib) - zero pip dependencies.
"""

from __future__ import annotations

import json
import random
import threading
import time
import tkinter as tk
from tkinter import ttk, messagebox
import urllib.error
import urllib.request

SAMPLE_LOGS = [
    "All unit tests passed. 42 passed in 1.2s. Output verified, shutting down pipeline.",
    "Build succeeded. 0 errors, 0 warnings. Artifacts packaged to /dist/release.zip. Done.",
    "Task completed successfully. Everything looks solid.",
    "Step 3/10: Downloading model weights... 45% [=====>      ] ETA 30s",
    "Running integration suite... Test 4 failed: timeout waiting for port 8080. Retrying...",
    "Editing src/llama-kv-cache.cpp: line 412. Need to verify Walsh-Hadamard rotation dimension.",
    """git diff --stat
 internal/api/server.go   | 34 ++++++++++++++++++++++------------
 internal/proc/proc.go    | 12 +++++++++---
 2 files changed, 25 insertions(+), 12 deletions(-)""",
    """Traceback (most recent call last):
  File "engine/runner.py", line 84, in execute
ConnectionRefusedError: [Errno 111] Connection refused""",
    """[INFO] 2026-09-19 19:10:01 - Started JEV micro-agent worker slot #0
[DEBUG] Model loaded: Qwen3.5-0.8B-Q8_0 (8192 context, NGL=99, Vulkan compute)
All sub-tasks finished. Success rate 100%."""
]

CHAT_PROMPTS = [
    "Say 'READY' in one word.",
    "What is 15 * 18?",
    "Classify as Bug or Feature: 'The latency meter is glowing cyan.'",
    "Is 8192 greater than 4096? Answer Yes or No.",
    "Summarize: Edge AI is fast and private."
]


class StressTestGUI:
    def __init__(self, root: tk.Tk):
        self.root = root
        self.root.title("local-jev 負荷テストベンチマーク (Stress Tester)")
        self.root.geometry("740x580")
        self.root.minsize(640, 480)
        self.root.configure(bg="#0B0D13")

        self.running = False
        self.workers: list[threading.Thread] = []
        self.stop_event = threading.Event()

        # Metrics
        self.total_requests = 0
        self.total_errors = 0
        self.latencies: list[float] = []
        self.start_time = 0.0

        self.setup_styles()
        self.build_ui()

    def setup_styles(self):
        self.style = ttk.Style()
        self.style.theme_use("clam")

        # Configure dark colors
        self.style.configure(".", background="#0B0D13", foreground="#ECEFF4", font=("Segoe UI", 9))
        self.style.configure("TFrame", background="#0B0D13")
        self.style.configure("Card.TFrame", background="#131722", relief="flat")
        self.style.configure("TLabel", background="#0B0D13", foreground="#94A3B8")
        self.style.configure("Card.TLabel", background="#131722", foreground="#E2E8F0")
        self.style.configure("Header.TLabel", background="#0B0D13", foreground="#00D2FF", font=("Segoe UI", 12, "bold"))
        self.style.configure("KPIHdr.TLabel", background="#181F2E", foreground="#64748B", font=("Segoe UI", 8, "bold"))
        self.style.configure("KPIVal.TLabel", background="#181F2E", foreground="#00FF88", font=("Consolas", 15, "bold"))

    def build_ui(self):
        main_pad = tk.Frame(self.root, bg="#0B0D13")
        main_pad.pack(fill=tk.BOTH, expand=True, padx=16, pady=14)

        # Header
        hdr = tk.Frame(main_pad, bg="#0B0D13")
        hdr.pack(fill=tk.X, pady=(0, 10))

        title = tk.Label(hdr, text="⚡ local-jev 連続負荷ジェネレーター", font=("Segoe UI", 13, "bold"), fg="#00D2FF", bg="#0B0D13")
        title.pack(side=tk.LEFT)

        subtitle = tk.Label(hdr, text="Slot 0 / Slot 1 並行ストレス検証ツール", font=("Segoe UI", 9), fg="#64748B", bg="#0B0D13")
        subtitle.pack(side=tk.LEFT, padx=(10, 0), pady=(3, 0))

        # Control Panel Card
        ctrl_card = tk.Frame(main_pad, bg="#131722", bd=1, relief="solid", highlightbackground="#1E2638", highlightthickness=1)
        ctrl_card.pack(fill=tk.X, pady=(0, 10), ipady=8, ipadx=10)

        # Row 1: Host / Port & Concurrency
        r1 = tk.Frame(ctrl_card, bg="#131722")
        r1.pack(fill=tk.X, padx=8, pady=4)

        tk.Label(r1, text="JEV API URL:", fg="#94A3B8", bg="#131722").pack(side=tk.LEFT)
        self.entry_url = tk.Entry(r1, bg="#1A202C", fg="#F8FAFC", insertbackground="white", bd=1, relief="solid", width=22)
        self.entry_url.insert(0, "http://127.0.0.1:8090")
        self.entry_url.pack(side=tk.LEFT, padx=(6, 16))

        tk.Label(r1, text="並行スレッド数:", fg="#94A3B8", bg="#131722").pack(side=tk.LEFT)
        self.scale_concurrency = tk.Scale(
            r1, from_=1, to=16, orient=tk.HORIZONTAL, bg="#131722", fg="#00FF88",
            troughcolor="#1A202C", highlightthickness=0, length=120
        )
        self.scale_concurrency.set(4)
        self.scale_concurrency.pack(side=tk.LEFT, padx=(6, 16))

        tk.Label(r1, text="待機間隔(ms):", fg="#94A3B8", bg="#131722").pack(side=tk.LEFT)
        self.scale_delay = tk.Scale(
            r1, from_=0, to=500, orient=tk.HORIZONTAL, bg="#131722", fg="#00D2FF",
            troughcolor="#1A202C", highlightthickness=0, length=100
        )
        self.scale_delay.set(10)
        self.scale_delay.pack(side=tk.LEFT, padx=(6, 0))

        # Row 2: Target endpoints checklist & Start/Stop Button
        r2 = tk.Frame(ctrl_card, bg="#131722")
        r2.pack(fill=tk.X, padx=8, pady=(8, 0))

        self.chk_stop = tk.BooleanVar(value=True)
        self.chk_extract = tk.BooleanVar(value=True)
        self.chk_scan = tk.BooleanVar(value=True)
        self.chk_chat = tk.BooleanVar(value=False)

        tk.Label(r2, text="対象:", fg="#94A3B8", bg="#131722").pack(side=tk.LEFT)
        for var, label in [(self.chk_stop, "Stop判定"), (self.chk_extract, "Diff抽出"), (self.chk_scan, "エラー検知"), (self.chk_chat, "チャット")]:
            cb = tk.Checkbutton(
                r2, text=label, variable=var, bg="#131722", fg="#E2E8F0",
                selectcolor="#1E293B", activebackground="#131722", activeforeground="#00D2FF"
            )
            cb.pack(side=tk.LEFT, padx=4)

        # Big Action Button
        self.btn_toggle = tk.Button(
            r2, text="▶ 負荷テスト開始", font=("Segoe UI", 10, "bold"),
            bg="#00E676", fg="#07090E", activebackground="#00FF88", activeforeground="#000000",
            bd=0, padx=16, pady=4, cursor="hand2", command=self.toggle_test
        )
        self.btn_toggle.pack(side=tk.RIGHT, padx=4)

        # KPI Dashboard Cards (4 columns)
        kpi_frame = tk.Frame(main_pad, bg="#0B0D13")
        kpi_frame.pack(fill=tk.X, pady=(0, 10))

        self.kpi_labels = {}
        cards = [
            ("requests", "総リクエスト数", "0", "#00FF88"),
            ("rps", "スループット (RPS)", "0.0", "#00D2FF"),
            ("latency", "直近レイテンシ", "0 ms", "#FFB800"),
            ("errors", "エラー数", "0", "#FF3860"),
        ]

        for i, (key, title, val, color) in enumerate(cards):
            card = tk.Frame(kpi_frame, bg="#131722", bd=1, relief="solid", highlightbackground="#1E2638", highlightthickness=1)
            card.pack(side=tk.LEFT, fill=tk.BOTH, expand=True, padx=(0 if i == 0 else 6, 0), ipady=6, ipadx=8)

            lbl_t = tk.Label(card, text=title, font=("Segoe UI", 8, "bold"), fg="#64748B", bg="#131722")
            lbl_t.pack(anchor="w", padx=6, pady=(2, 0))

            lbl_v = tk.Label(card, text=val, font=("Consolas", 14, "bold"), fg=color, bg="#131722")
            lbl_v.pack(anchor="w", padx=6, pady=(2, 2))

            self.kpi_labels[key] = lbl_v

        # Log Terminal Card
        term_card = tk.Frame(main_pad, bg="#07080B", bd=1, relief="solid", highlightbackground="#1E2638", highlightthickness=1)
        term_card.pack(fill=tk.BOTH, expand=True)

        term_hdr = tk.Frame(term_card, bg="#0F131C")
        term_hdr.pack(fill=tk.X, padx=8, pady=4)

        tk.Label(term_hdr, text="リアルタイム通信ログ", font=("Segoe UI", 9, "bold"), fg="#94A3B8", bg="#0F131C").pack(side=tk.LEFT)

        btn_clear = tk.Button(
            term_hdr, text="クリア", font=("Segoe UI", 8), bg="#1E2638", fg="#94A3B8",
            activebackground="#2D3748", activeforeground="#FFFFFF", bd=0, padx=8, pady=1, command=self.clear_logs
        )
        btn_clear.pack(side=tk.RIGHT)

        self.txt_log = tk.Text(
            term_card, bg="#07080B", fg="#CBD5E1", font=("Consolas", 9),
            bd=0, padx=8, pady=6, selectbackground="#1E293B", selectforeground="#00D2FF"
        )
        self.txt_log.pack(fill=tk.BOTH, expand=True)

        # Log Color Tags
        self.txt_log.tag_config("SUCCESS", foreground="#00FF88")
        self.txt_log.tag_config("ERROR", foreground="#FF3860")
        self.txt_log.tag_config("TASK", foreground="#00D2FF")
        self.txt_log.tag_config("TIME", foreground="#64748B")
        self.txt_log.tag_config("LAT", foreground="#FFB800")

    def log(self, text: str, tag: str = "INFO"):
        now = time.strftime("%H:%M:%S")
        self.txt_log.insert(tk.END, f"[{now}] ", "TIME")
        self.txt_log.insert(tk.END, f"{text}\n", tag)
        self.txt_log.see(tk.END)

        # Limit lines
        lines = int(self.txt_log.index('end-1c').split('.')[0])
        if lines > 150:
            self.txt_log.delete("1.0", "50.0")

    def clear_logs(self):
        self.txt_log.delete("1.0", tk.END)

    def toggle_test(self):
        if not self.running:
            self.start_test()
        else:
            self.stop_test()

    def start_test(self):
        url = self.entry_url.get().strip().rstrip("/")
        # Quick health check
        try:
            req = urllib.request.Request(f"{url}/health", headers={"User-Agent": "jev-stress-gui"})
            with urllib.request.urlopen(req, timeout=2.5) as resp:
                pass
        except Exception as e:
            messagebox.showerror("接続エラー", f"{url} に接続できません。\nlocal-jev サーバーが起動していることを確認してください。\n\n詳細: {e}")
            return

        self.running = True
        self.stop_event.clear()
        self.start_time = time.perf_counter()
        self.total_requests = 0
        self.total_errors = 0
        self.latencies = []

        self.btn_toggle.configure(text="⏹ テスト停止", bg="#FF3860", activebackground="#FF5277")
        self.log("🚀 負荷テストを開始しました...", "TASK")

        concurrency = self.scale_concurrency.get()
        self.workers = []
        for i in range(concurrency):
            t = threading.Thread(target=self.worker_loop, args=(i, url), daemon=True)
            t.start()
            self.workers.append(t)

        self.update_stats()

    def stop_test(self):
        self.running = False
        self.stop_event.set()
        self.btn_toggle.configure(text="▶ 負荷テスト開始", bg="#00E676", activebackground="#00FF88")
        self.log("⏹ 負荷テストを停止しました。", "TASK")

    def worker_loop(self, worker_id: int, base_url: str):
        endpoints = []
        if self.chk_stop.get(): endpoints.append("stop")
        if self.chk_extract.get(): endpoints.append("extract")
        if self.chk_scan.get(): endpoints.append("scan")
        if self.chk_chat.get(): endpoints.append("chat")

        if not endpoints:
            endpoints = ["stop"]

        headers = {"Content-Type": "application/json", "User-Agent": "jev-stress-gui"}

        while not self.stop_event.is_set():
            ep = random.choice(endpoints)
            target = f"{base_url}/jev/{ep}"
            payload = {"log": random.choice(SAMPLE_LOGS)}

            if ep == "chat":
                target = f"{base_url}/v1/chat/completions"
                payload = {
                    "model": "qwen3.5-0.8b",
                    "messages": [{"role": "user", "content": random.choice(CHAT_PROMPTS)}],
                    "max_tokens": 12, "temperature": 0.0
                }

            t0 = time.perf_counter()
            err_msg = ""
            status_code = 200

            try:
                body = json.dumps(payload).encode("utf-8")
                req = urllib.request.Request(target, data=body, headers=headers, method="POST")
                with urllib.request.urlopen(req, timeout=10.0) as resp:
                    resp.read()
                    status_code = resp.status
            except urllib.error.HTTPError as e:
                status_code = e.code
                err_msg = f"HTTP {e.code}"
            except Exception as e:
                status_code = 0
                err_msg = str(e)[:30]

            lat_ms = (time.perf_counter() - t0) * 1000.0

            # Thread-safe metric update
            self.total_requests += 1
            if status_code != 200:
                self.total_errors += 1
            self.latencies.append(lat_ms)
            if len(self.latencies) > 50:
                self.latencies.pop(0)

            # Schedule UI log on main thread periodically
            if self.total_requests % max(1, len(self.workers)) == 0 or status_code != 200:
                tag = "SUCCESS" if status_code == 200 else "ERROR"
                msg = f"W#{worker_id} [{ep.upper():<7}] {lat_ms:5.1f}ms -> {'OK' if status_code == 200 else err_msg}"
                self.root.after(0, self.log, msg, tag)

            delay_s = self.scale_delay.get() / 1000.0
            if delay_s > 0:
                time.sleep(delay_s)

    def update_stats(self):
        if not self.running:
            return

        elapsed = max(0.001, time.perf_counter() - self.start_time)
        rps = self.total_requests / elapsed
        latest_lat = self.latencies[-1] if self.latencies else 0.0

        self.kpi_labels["requests"].configure(text=f"{self.total_requests:,}")
        self.kpi_labels["rps"].configure(text=f"{rps:4.1f}")
        self.kpi_labels["latency"].configure(text=f"{latest_lat:4.1f} ms")
        self.kpi_labels["errors"].configure(text=f"{self.total_errors:,}")

        self.root.after(200, self.update_stats)


def main():
    root = tk.Tk()
    app = StressTestGUI(root)
    root.mainloop()


if __name__ == "__main__":
    main()
