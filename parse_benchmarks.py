#!/usr/bin/env python3
import re
from collections import defaultdict

def parse_benchmark_file(filepath):
    """Parse a Go benchmark file and extract metrics per test."""
    results = defaultdict(lambda: {"ns_ops": [], "b_op": None, "allocs_op": None})

    with open(filepath, 'r') as f:
        for line in f:
            # Match benchmark lines like:
            # BenchmarkTruncateAll/ASCII_short_10B-12         	24526083	        49.33 ns/op	      48 B/op	       3 allocs/op
            match = re.match(
                r'BenchmarkTruncateAll/(\S+)-\d+\s+\d+\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op',
                line
            )
            if match:
                test_name = match.group(1)
                ns_op = float(match.group(2))
                b_op = int(match.group(3))
                allocs_op = int(match.group(4))

                results[test_name]["ns_ops"].append(ns_op)
                results[test_name]["b_op"] = b_op
                results[test_name]["allocs_op"] = allocs_op

    return results

def format_table(results, title):
    """Format results as a table."""
    lines = []
    lines.append(f"\n{'=' * 90}")
    lines.append(f"{title}")
    lines.append(f"{'=' * 90}")

    header = f"{'Test Name':<30} {'min-ns/op':>12} {'max-ns/op':>12} {'avg-ns/op':>12} {'B/op':>8} {'allocs/op':>10}"
    lines.append(header)
    lines.append("-" * 90)

    for test_name in sorted(results.keys()):
        data = results[test_name]
        ns_ops = data["ns_ops"]
        if ns_ops:
            min_ns = min(ns_ops)
            max_ns = max(ns_ops)
            avg_ns = sum(ns_ops) / len(ns_ops)
            b_op = data["b_op"]
            allocs_op = data["allocs_op"]

            line = f"{test_name:<30} {min_ns:>12.2f} {max_ns:>12.2f} {avg_ns:>12.2f} {b_op:>8} {allocs_op:>10}"
            lines.append(line)

    return "\n".join(lines)

def format_comparison_table(old_results, new_results):
    """Format a comparison table showing old vs new performance."""
    lines = []
    lines.append(f"\n{'=' * 110}")
    lines.append("COMPARISON: OLD (no UTF-8) vs NEW (with UTF-8)")
    lines.append(f"{'=' * 110}")

    header = f"{'Test Name':<30} {'Old avg':>10} {'New avg':>10} {'Diff':>10} {'Diff %':>10} {'B/op':>8} {'allocs/op':>10}"
    lines.append(header)
    lines.append("-" * 110)

    for test_name in sorted(new_results.keys()):
        old_data = old_results.get(test_name, {"ns_ops": []})
        new_data = new_results[test_name]

        old_ns_ops = old_data["ns_ops"]
        new_ns_ops = new_data["ns_ops"]

        if old_ns_ops and new_ns_ops:
            old_avg = sum(old_ns_ops) / len(old_ns_ops)
            new_avg = sum(new_ns_ops) / len(new_ns_ops)
            diff = new_avg - old_avg
            diff_pct = (diff / old_avg) * 100 if old_avg > 0 else 0
            b_op = new_data["b_op"]
            allocs_op = new_data["allocs_op"]

            line = f"{test_name:<30} {old_avg:>10.2f} {new_avg:>10.2f} {diff:>+10.2f} {diff_pct:>+9.1f}% {b_op:>8} {allocs_op:>10}"
            lines.append(line)

    return "\n".join(lines)

def main():
    old_file = "benchmark_old_no_utf8.txt"
    new_file = "benchmark_new_utf8.txt"

    old_results = parse_benchmark_file(old_file)
    new_results = parse_benchmark_file(new_file)

    output = []
    output.append("BENCHMARK RESULTS ANALYSIS")
    output.append("=" * 90)
    output.append(f"Old implementation: {old_file}")
    output.append(f"New implementation: {new_file}")

    output.append(format_table(old_results, "OLD IMPLEMENTATION (no UTF-8 handling)"))
    output.append(format_table(new_results, "NEW IMPLEMENTATION (with UTF-8 handling)"))
    output.append(format_comparison_table(old_results, new_results))

    result = "\n".join(output)

    # Write to file
    with open("benchmark_analysis.txt", "w") as f:
        f.write(result)

    print(result)
    print("\n\nResults saved to benchmark_analysis.txt")

if __name__ == "__main__":
    main()
