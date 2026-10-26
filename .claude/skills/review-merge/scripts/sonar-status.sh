#!/usr/bin/env bash
# PR の SonarQube Cloud の結果（Quality Gate、落ちた条件、未解決の指摘、新しいコードのカバレッジが低いファイル）を表示する。
# 使い方: sonar-status.sh <PR 番号>
set -euo pipefail
pr="${1:?PR 番号を渡す}"
project=zono0013_codegen
api=https://sonarcloud.io/api

# 解析がまだ終わっていない（fork からの PR では解析されない）と 404 になる
code=$(curl -sS -o /dev/null -w '%{http_code}' "$api/qualitygates/project_status?projectKey=$project&pullRequest=$pr")
if [ "$code" = 404 ]; then
  echo "PR #$pr の解析がまだ無い（終わっていないか、fork からの PR で解析されない）"
  exit 2
fi
curl -fsS "$api/qualitygates/project_status?projectKey=$project&pullRequest=$pr" | python3 -c '
import json, sys
d = json.load(sys.stdin)["projectStatus"]
print("Quality Gate:", d["status"])
for c in d["conditions"]:
    if c["status"] != "OK":
        print("  NG", c["metricKey"], c.get("actualValue"), "(しきい値", c.get("errorThreshold"), ")")
'
curl -fsS "$api/issues/search?componentKeys=$project&pullRequest=$pr&resolved=false&ps=100" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print("未解決の指摘:", d["total"])
for i in d["issues"]:
    print("  ", i["severity"], i["rule"], i["component"].split(":", 1)[-1] + ":" + str(i.get("line", "")), i["message"])
'
curl -fsS "$api/hotspots/search?projectKey=$project&pullRequest=$pr&status=TO_REVIEW" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print("未確認のセキュリティホットスポット:", d["paging"]["total"])
for h in d["hotspots"]:
    print("  ", h["component"].split(":", 1)[-1] + ":" + str(h.get("line", "")), h["message"])
'
curl -fsS "$api/measures/component_tree?component=$project&pullRequest=$pr&metricKeys=new_coverage,new_uncovered_lines&qualifiers=FIL&ps=500" | python3 -c '
import json, sys
d = json.load(sys.stdin)
rows = []
for c in d.get("components", []):
    m = {x["metric"]: (x.get("periods") or [{}])[0].get("value", x.get("value")) for x in c["measures"]}
    if m.get("new_uncovered_lines") not in (None, "0"):
        rows.append((c["path"], m.get("new_coverage"), m.get("new_uncovered_lines")))
if rows:
    print("新しいコードで、テストが通っていない行があるファイル:")
    for path, cov, lines in sorted(rows):
        print("  ", path, "カバレッジ", cov, "未カバー", lines, "行")
'
