import fs from 'node:fs';
import path from 'node:path';

const inputPath = process.argv[2];
if (!inputPath) {
  console.error('Usage: node summarize-playwright-json.mjs <playwright-json-report> [output-json]');
  process.exit(2);
}

const outputPath = process.argv[3] || '';
const raw = fs.readFileSync(inputPath, 'utf8');
const report = JSON.parse(raw);

const stats = report.stats || {};
const expected = Number(stats.expected || 0);
const unexpected = Number(stats.unexpected || 0);
const flaky = Number(stats.flaky || 0);
const skipped = Number(stats.skipped || 0);
const interrupted = Number(stats.interrupted || 0);
const total = expected + unexpected + flaky + skipped + interrupted;
const passed = expected;
const failed = unexpected;
const passRate = total > 0 ? passed / total : 0;

const summary = {
  generated_at: new Date().toISOString(),
  report_path: path.resolve(inputPath),
  totals: {
    total,
    passed,
    failed,
    flaky,
    skipped,
    interrupted,
  },
  pass_rate: Number(passRate.toFixed(6)),
  benchmark_first_try_success_rate: Number(passRate.toFixed(6)),
  thresholds: {
    required_pass_rate: process.env.RELIABILITY_REQUIRED_PASS_RATE ? Number(process.env.RELIABILITY_REQUIRED_PASS_RATE) : null,
    expected_min_tests: process.env.RELIABILITY_EXPECTED_MIN_TESTS ? Number(process.env.RELIABILITY_EXPECTED_MIN_TESTS) : null,
  },
};

if (outputPath) {
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, JSON.stringify(summary, null, 2));
}

console.log(JSON.stringify(summary, null, 2));

const requiredPassRate = summary.thresholds.required_pass_rate;
if (typeof requiredPassRate === 'number' && !Number.isNaN(requiredPassRate) && passRate < requiredPassRate) {
  console.error(`Reliability gate failed: pass_rate ${passRate.toFixed(4)} < required ${requiredPassRate.toFixed(4)}`);
  process.exit(1);
}

const expectedMinTests = summary.thresholds.expected_min_tests;
if (typeof expectedMinTests === 'number' && !Number.isNaN(expectedMinTests) && total < expectedMinTests) {
  console.error(`Reliability gate failed: total tests ${total} < expected minimum ${expectedMinTests}`);
  process.exit(1);
}
