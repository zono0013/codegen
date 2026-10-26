#!/usr/bin/env python3
"""CI の Submodule E2E（.github/workflows/ci.yml の submodule-e2e ジョブ）をローカルで再現する。

コミット済みのブランチを、空のリポジトリに Submodule として取り込み、ジョブの run を順に bash -e で実行する。
`uses:` のステップ（checkout / setup-go など）は飛ばすので、Go / Node / pnpm はローカルのものを使う。
CI は PR を main にマージしたコミットを試すので、main が先に進んでいるなら rebase してから使う。

作業ディレクトリは一時ディレクトリに作り、最後にパスを表示する（中身を見て原因を追えるよう消さない。不要なら rm -rf する）。

使い方: e2e-local.py [ブランチ名]   # 省略時は今のブランチ
"""
import os
import re
import shlex
import subprocess
import sys
import tempfile
import textwrap

STEP_INDENT = " " * 6  # jobs.<job>.steps の各要素の "- " の位置
KEY_INDENT = " " * 8   # ステップのキーの位置


def git(*args, cwd):
    return subprocess.check_output(["git", *args], text=True, cwd=cwd).strip()


def job_text(ci):
    start = ci.index("\n  submodule-e2e:\n")
    nxt = re.search(r"\n  [A-Za-z][\w-]*:\n", ci[start + 1:])
    return ci[start:start + 1 + nxt.start()] if nxt else ci[start:]


def parse_steps(job):
    """steps の各要素を {name, working-directory, run, uses, ...} にする。読めない書き方はエラーで止める。"""
    body = job[job.index("\n    steps:\n") + len("\n    steps:\n"):]
    blocks = re.split(rf"^{STEP_INDENT}- ", body, flags=re.M)[1:]
    steps = []
    for block in blocks:
        lines = (KEY_INDENT + block).split("\n")
        step, i = {}, 0
        while i < len(lines):
            line = lines[i]
            m = re.match(rf"^{KEY_INDENT}([\w-]+):\s?(.*)$", line)
            if not m:
                i += 1
                continue
            key, value = m.group(1), m.group(2)
            if value in ("|", ">"):
                block_lines = []
                i += 1
                while i < len(lines) and (lines[i].startswith(KEY_INDENT + "  ") or not lines[i].strip()):
                    block_lines.append(lines[i])
                    i += 1
                step[key] = textwrap.dedent("\n".join(block_lines)).strip("\n") + "\n"
                continue
            step[key] = value
            i += 1
        steps.append(step)
    return steps


def main():
    repo = git("rev-parse", "--show-toplevel", cwd=os.getcwd())
    branch = sys.argv[1] if len(sys.argv) > 1 else git("rev-parse", "--abbrev-ref", "HEAD", cwd=repo)
    if branch == "HEAD":
        sys.exit("detached HEAD では使えない。ブランチ名を渡す")
    if git("status", "--porcelain", cwd=repo):
        print("注意: コミットしていない変更がある。E2E はコミット済みの内容で動く", file=sys.stderr)

    with open(os.path.join(repo, ".github/workflows/ci.yml"), encoding="utf-8") as f:
        steps = parse_steps(job_text(f.read()))
    for s in steps:
        unsupported = set(s) - {"name", "working-directory", "run", "uses", "with"}
        if unsupported:
            sys.exit(f"このスクリプトが扱えないキーがある: {sorted(unsupported)}（{s.get('name', s)}）。スクリプトを直す")
        if "run" in s and "${{" in s["run"]:
            sys.exit(f"run の中の ${{{{ }}}} は再現できない: {s.get('name')}。スクリプトを直す")
        if "uses" not in s and "run" not in s:
            sys.exit(f"run も uses も無いステップ: {s}")

    work = tempfile.mkdtemp(prefix="codegen-e2e-")
    print("作業ディレクトリ:", work)
    env = dict(os.environ, GITHUB_WORKSPACE=work)
    ran = 0
    for s in steps:
        if "uses" in s:
            continue
        name = s.get("name", s["run"].splitlines()[0])
        print(f"\n### {name}", flush=True)
        if "submodule add" in s["run"]:
            # CI は checkout したソースを取り込む。ローカルではリポジトリのブランチを取り込む
            script = ("git init -q -b main app && cd app && git -c protocol.file.allow=always submodule add -q -b "
                      f"{shlex.quote(branch)} {shlex.quote(repo)} codegen")
        else:
            script = s["run"]
        cwd = os.path.join(work, s["working-directory"]) if "working-directory" in s else work
        if subprocess.run(["bash", "-e", "-c", script], cwd=cwd, env=env, stdin=subprocess.DEVNULL).returncode != 0:
            print(f"\n✗ 失敗: {name}（作業ディレクトリ: {work}）")
            sys.exit(1)
        ran += 1
    print(f"\n✓ Submodule E2E の {ran} ステップがすべて通った（作業ディレクトリ: {work}）")


if __name__ == "__main__":
    main()
