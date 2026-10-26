#!/usr/bin/env python3
"""CI の Submodule E2E（.github/workflows/ci.yml の submodule-e2e ジョブ）をローカルで再現する。

コミット済みのブランチを、空のリポジトリに Submodule として取り込み、ジョブの run を順に bash -e で実行する。
作業ディレクトリは一時ディレクトリに作り、最後にパスを表示する（消さない。中身を見て原因を追えるように）。

使い方: e2e-local.py [ブランチ名]   # 省略時は今のブランチ
"""
import os
import re
import subprocess
import sys
import tempfile
import textwrap

repo = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
branch = sys.argv[1] if len(sys.argv) > 1 else subprocess.check_output(
    ["git", "rev-parse", "--abbrev-ref", "HEAD"], text=True, cwd=repo).strip()
if subprocess.check_output(["git", "status", "--porcelain"], text=True, cwd=repo).strip():
    print("注意: コミットしていない変更がある。E2E はコミット済みの内容で動く", file=sys.stderr)

ci = open(os.path.join(repo, ".github/workflows/ci.yml"), encoding="utf-8").read()
start = ci.index("  submodule-e2e:")
nxt = re.search(r"\n  [a-z][\w-]*:\n", ci[start + 1:])
job = ci[start:start + 1 + nxt.start()] if nxt else ci[start:]
steps = re.findall(
    r"      - name: (.*?)\n(?:        working-directory: (.*?)\n)?        run: \|\n(.*?)(?=\n      - |\Z)", job, re.S)

work = tempfile.mkdtemp(prefix="codegen-e2e-")
print("作業ディレクトリ:", work)
for name, wd, body in steps:
    print(f"\n### {name}", flush=True)
    if "Submodule として追加" in name:
        script = ("git init -q -b main app && cd app && "
                  f"git -c protocol.file.allow=always submodule add -q -b {branch} {repo} codegen")
    else:
        script = textwrap.dedent(body)
    cwd = os.path.join(work, wd) if wd else work
    env = dict(os.environ, GITHUB_WORKSPACE=work)
    if subprocess.run(["bash", "-e", "-c", script], cwd=cwd, env=env, stdin=subprocess.DEVNULL).returncode != 0:
        print(f"\n✗ 失敗: {name}（作業ディレクトリ: {work}）")
        sys.exit(1)
print(f"\n✓ Submodule E2E の全ステップが通った（作業ディレクトリ: {work}）")
