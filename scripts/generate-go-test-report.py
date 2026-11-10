import json
import sys
from collections import defaultdict
from pathlib import Path


def read_events(paths):
    for path in paths:
        with path.open(encoding="utf-8") as stream:
            for line_number, line in enumerate(stream, 1):
                try:
                    yield json.loads(line)
                except json.JSONDecodeError:
                    print(f"Skipping invalid JSON in {path}:{line_number}", file=sys.stderr)


def main():
    output_dir = Path(sys.argv[1] if len(sys.argv) > 1 else "test-results")
    inputs = [Path(value) for value in sys.argv[2:]]
    packages = defaultdict(lambda: {"pass": 0, "fail": 0, "skip": 0, "output": []})
    failures = []

    for event in read_events(inputs):
        if event.get("Action") not in {"pass", "fail", "skip"}:
            continue
        package = event.get("Package", "unknown")
        test = event.get("Test")
        if test:
            packages[package][event["Action"]] += 1
        elif event["Action"] == "fail":
            packages[package]["fail"] += 1
            test = "package setup or build"
        if event["Action"] == "fail":
            failures.append((package, test))
        if event.get("Output") and event["Action"] == "fail":
            packages[package]["output"].append(event["Output"].rstrip())

    output_dir.mkdir(parents=True, exist_ok=True)
    total = {key: sum(value[key] for value in packages.values()) for key in ("pass", "fail", "skip")}
    report = [
        "# Go Test Report",
        "",
        "This report is generated from `go test -json` output and is intended to support reproducible bug reports.",
        "",
        f"- **Passed:** {total['pass']}",
        f"- **Failed:** {total['fail']}",
        f"- **Skipped:** {total['skip']}",
        "",
        "## Package results",
        "",
        "| Package | Passed | Failed | Skipped |",
        "|---|---:|---:|---:|",
    ]
    for package in sorted(packages):
        result = packages[package]
        report.append(f"| `{package}` | {result['pass']} | {result['fail']} | {result['skip']} |")

    if failures:
        report.extend(["", "## Failed tests", ""])
        for package, test in failures:
            report.append(f"- `{package}` - `{test}`")
        report.extend(["", "Record the commit SHA, workflow URL, environment, request data, expected result, actual result, and logs when filing each bug."])
    else:
        report.extend(["", "No test failures were reported."])

    (output_dir / "go-test-report.md").write_text("\n".join(report) + "\n", encoding="utf-8")
    (output_dir / "go-test-summary.json").write_text(json.dumps({"totals": total, "failures": failures}, indent=2) + "\n", encoding="utf-8")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
