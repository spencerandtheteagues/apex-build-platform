import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const inputPath = process.argv[2];
if (!inputPath) {
  console.error('Usage: node summarize-playwright-json.mjs <playwright-json-report> [output-json]');
  process.exit(2);
}

const outputPath = process.argv[3] || '';
const raw = fs.readFileSync(inputPath, 'utf8');
const report = JSON.parse(raw);

// --- Global stats ---
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

// --- Per-suite breakdown ---
// Parse individual test results and group by suite prefix (health:, auth:, preflight:, etc.)
const suiteResults = {};

function walkSuites(suites) {
  for (const suite of suites || []) {
    for (const spec of suite.specs || []) {
      for (const test of spec.tests || []) {
        const title = spec.title || '';
        const prefix = extractSuitePrefix(title);
        if (!suiteResults[prefix]) {
          suiteResults[prefix] = { total: 0, passed: 0, failed: 0, tests: [] };
        }
        const ok = test.status === 'expected';
        suiteResults[prefix].total++;
        if (ok) suiteResults[prefix].passed++;
        else suiteResults[prefix].failed++;
        suiteResults[prefix].tests.push({ title, status: test.status });
      }
    }
    walkSuites(suite.suites);
  }
}

function extractSuitePrefix(title) {
  const match = title.match(/^([\w-]+):/);
  if (match) return match[1];
  if (title.startsWith('preview-proxy:')) return 'preview-proxy';
  return 'other';
}

walkSuites(report.suites);

const suitePassRates = {};
for (const [prefix, data] of Object.entries(suiteResults)) {
  suitePassRates[prefix] = {
    total: data.total,
    passed: data.passed,
    failed: data.failed,
    pass_rate: data.total > 0 ? Number((data.passed / data.total).toFixed(6)) : 0,
  };
}

// --- Load SLO thresholds ---
const sloPath = path.resolve(__dirname, '..', 'slo-thresholds.json');
let sloConfig = null;
try {
  sloConfig = JSON.parse(fs.readFileSync(sloPath, 'utf8'));
} catch {
  // SLO config is optional; fall back to env-based thresholds
}

// Merge env overrides with SLO config
const requiredPassRate = process.env.RELIABILITY_REQUIRED_PASS_RATE
  ? Number(process.env.RELIABILITY_REQUIRED_PASS_RATE)
  : sloConfig?.global?.min_pass_rate ?? null;
const expectedMinTests = process.env.RELIABILITY_EXPECTED_MIN_TESTS
  ? Number(process.env.RELIABILITY_EXPECTED_MIN_TESTS)
  : sloConfig?.global?.min_total_tests ?? null;

// --- Build summary ---
const summary = {
  generated_at: new Date().toISOString(),
  report_path: path.resolve(inputPath),
  totals: { total, passed, failed, flaky, skipped, interrupted },
  pass_rate: Number(passRate.toFixed(6)),
  benchmark_first_try_success_rate: Number(passRate.toFixed(6)),
  per_suite: suitePassRates,
  thresholds: {
    required_pass_rate: requiredPassRate,
    expected_min_tests: expectedMinTests,
  },
  slo_violations: [],
};

// --- Enforce global thresholds ---
if (typeof requiredPassRate === 'number' && !Number.isNaN(requiredPassRate) && passRate < requiredPassRate) {
  summary.slo_violations.push({
    scope: 'global',
    metric: 'pass_rate',
    actual: Number(passRate.toFixed(6)),
    required: requiredPassRate,
    message: `Global pass rate ${passRate.toFixed(4)} < required ${requiredPassRate.toFixed(4)}`,
  });
}

if (typeof expectedMinTests === 'number' && !Number.isNaN(expectedMinTests) && total < expectedMinTests) {
  summary.slo_violations.push({
    scope: 'global',
    metric: 'min_tests',
    actual: total,
    required: expectedMinTests,
    message: `Total tests ${total} < expected minimum ${expectedMinTests}`,
  });
}

// --- Enforce per-suite SLO thresholds ---
if (sloConfig?.per_suite) {
  for (const [suiteKey, threshold] of Object.entries(sloConfig.per_suite)) {
    const suiteData = suitePassRates[suiteKey];
    if (!suiteData) continue; // suite not present in results; skip
    const minRate = threshold.min_pass_rate;
    if (typeof minRate === 'number' && suiteData.pass_rate < minRate) {
      summary.slo_violations.push({
        scope: `suite:${suiteKey}`,
        metric: 'pass_rate',
        actual: suiteData.pass_rate,
        required: minRate,
        description: threshold.description || '',
        message: `Suite "${suiteKey}" pass rate ${suiteData.pass_rate.toFixed(4)} < required ${minRate.toFixed(4)}`,
      });
    }
  }
}

summary.slo_passed = summary.slo_violations.length === 0;

// --- Output ---
if (outputPath) {
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, JSON.stringify(summary, null, 2));
}

console.log(JSON.stringify(summary, null, 2));

// --- Exit with failure if any SLO violated ---
if (summary.slo_violations.length > 0) {
  console.error(`\nSLO GATE FAILED: ${summary.slo_violations.length} violation(s)`);
  for (const v of summary.slo_violations) {
    console.error(`  - ${v.message}`);
  }
  process.exit(1);
}
