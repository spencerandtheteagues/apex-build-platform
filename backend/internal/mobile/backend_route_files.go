package mobile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func backendContractSourceFiles(spec MobileAppSpec) []SourceFile {
	if !shouldGenerateMobileBackendSource(spec) {
		return nil
	}

	return []SourceFile{
		sourceFile("backend/package.json", mobileBackendPackageJSON(spec), "json"),
		sourceFile("backend/tsconfig.json", mobileBackendTSConfig(), "json"),
		sourceFile("backend/.env.example", mobileBackendEnvExample(), "dotenv"),
		sourceFile("backend/README.md", mobileBackendReadme(spec), "markdown"),
		sourceFile("backend/src/server.ts", mobileBackendServerTS(), "typescript"),
		sourceFile("backend/src/authAdapter.ts", mobileBackendAuthAdapterTS(), "typescript"),
		sourceFile("backend/src/persistenceAdapter.ts", mobileBackendPersistenceAdapterTS(spec), "typescript"),
		sourceFile("backend/src/uploadAdapter.ts", mobileBackendUploadAdapterTS(), "typescript"),
		sourceFile("backend/src/mobileContractRoutes.ts", mobileBackendRoutesTS(spec), "typescript"),
		sourceFile("docs/mobile-backend-routes.md", MobileBackendRoutesMarkdown(spec), "markdown"),
	}
}

func shouldGenerateMobileBackendSource(spec MobileAppSpec) bool {
	switch spec.Architecture.BackendMode {
	case BackendNewGenerated, BackendExistingApexGenerated:
		return len(spec.APIContracts) > 0
	default:
		return false
	}
}

func mobileBackendPackageJSON(spec MobileAppSpec) string {
	name := strings.TrimSpace(spec.App.Slug)
	if name == "" {
		name = "apex-mobile-app"
	}
	payload := map[string]any{
		"name":    name + "-backend",
		"version": manifestDefaultString(spec.Identity.Version, "1.0.0"),
		"private": true,
		"type":    "module",
		"scripts": map[string]string{
			"build":     "tsc -p tsconfig.json",
			"dev":       "tsx src/server.ts",
			"start":     "node dist/server.js",
			"typecheck": "tsc --noEmit",
		},
		"dependencies": map[string]string{},
		"devDependencies": map[string]string{
			"@types/node": "^22.15.0",
			"tsx":         "^4.20.0",
			"typescript":  "~5.9.0",
		},
	}
	encoded, _ := json.MarshalIndent(payload, "", "  ")
	return string(encoded) + "\n"
}

func mobileBackendTSConfig() string {
	return `{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022"],
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "forceConsistentCasingInFileNames": true,
    "skipLibCheck": true
  },
  "include": ["src/**/*.ts"]
}
`
}

func mobileBackendEnvExample() string {
	return `PORT=8080
MOBILE_API_ALLOWED_ORIGIN=*
APEX_MOBILE_DEMO_TOKEN=dev-mobile-token
`
}

func mobileBackendReadme(spec MobileAppSpec) string {
	return fmt.Sprintf(`# %s Backend Starter

This backend is generated from `+"`MobileAppSpec.APIContracts`"+` and matches the Expo mobile helpers in `+"`mobile/src/api/endpoints.ts`"+`.

It is a runnable local starter for mobile integration testing. It uses in-memory/demo data by default so exports are immediately testable outside Apex-Build. Replace the route handlers with your production persistence, auth, file storage, and notification services before launch.

## Run

`+"```bash"+`
npm install
npm run dev
`+"```"+`

Then set the mobile app environment:

`+"```bash"+`
EXPO_PUBLIC_API_BASE_URL=http://localhost:8080
`+"```"+`

## Contract

- Source manifest: `+"`mobile/docs/api-contract.json`"+`
- Route implementation: `+"`backend/src/mobileContractRoutes.ts`"+`
- Auth adapter: `+"`backend/src/authAdapter.ts`"+`
- Persistence adapter: `+"`backend/src/persistenceAdapter.ts`"+`
- Upload adapter: `+"`backend/src/uploadAdapter.ts`"+`
- Route docs: `+"`docs/mobile-backend-routes.md`"+`

Do not treat this backend starter as store-submission proof. Native builds, credentials, store metadata, App Store review, and Google Play review remain separate statuses.
`, manifestDefaultString(spec.App.Name, "Generated Mobile App"))
}

func mobileBackendServerTS() string {
	return `import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { authenticateMobileRequest } from './authAdapter.js';
import { mobileContractRoutes, type ContractRoute } from './mobileContractRoutes.js';

const port = Number(process.env.PORT ?? 8080);
const allowedOrigin = process.env.MOBILE_API_ALLOWED_ORIGIN ?? '*';

type MatchedRoute = {
  route: ContractRoute;
  params: Record<string, string>;
};

class HTTPError extends Error {
  constructor(readonly status: number, readonly code: string, message: string) {
    super(message);
  }
}

const server = createServer(async (req, res) => {
  setCORSHeaders(res);

  if (req.method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }

  const method = (req.method ?? 'GET').toUpperCase();
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (method === 'GET' && (url.pathname === '/healthz' || url.pathname === '/health')) {
    writeJSON(res, 200, { ok: true, service: 'apex-generated-mobile-backend' });
    return;
  }

  const matched = matchRoute(method, url.pathname);
  if (!matched) {
    writeJSON(res, 404, { error: 'Route not found', code: 'ROUTE_NOT_FOUND', path: url.pathname });
    return;
  }

  const auth = authenticateMobileRequest(req.headers);
  if (matched.route.authRequired && !auth) {
    writeJSON(res, 401, { error: 'Missing bearer token', code: 'MISSING_BEARER_TOKEN' });
    return;
  }

  try {
    const rawBody = await readRawBody(req);
    const body = parseBody(req, rawBody);
    const result = await matched.route.handle({
      params: matched.params,
      body,
      rawBody,
      headers: req.headers,
      auth
    });
    writeRouteResult(res, result);
  } catch (error) {
    if (error instanceof HTTPError) {
      writeJSON(res, error.status, { error: error.message, code: error.code });
      return;
    }
    const message = error instanceof Error ? error.message : 'Unhandled mobile backend error';
    writeJSON(res, 500, { error: message, code: 'MOBILE_BACKEND_ERROR' });
  }
});

server.listen(port, () => {
  console.log('Mobile API backend listening on http://localhost:' + port);
});

function matchRoute(method: string, pathname: string): MatchedRoute | null {
  for (const route of mobileContractRoutes) {
    if (route.method !== method) continue;
    const params = route.match(pathname);
    if (params) return { route, params };
  }
  return null;
}

function readRawBody(req: IncomingMessage): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('error', reject);
    req.on('end', () => resolve(Buffer.concat(chunks)));
  });
}

function parseBody(req: IncomingMessage, rawBody: Buffer): unknown {
  if (rawBody.length === 0) return undefined;
  const contentType = req.headers['content-type'] ?? '';
  if (Array.isArray(contentType) ? contentType.some((value) => value.includes('application/json')) : contentType.includes('application/json')) {
    try {
      return JSON.parse(rawBody.toString('utf8'));
    } catch {
      throw new HTTPError(400, 'INVALID_JSON_BODY', 'Invalid JSON request body');
    }
  }
  return { byteLength: rawBody.length, contentType };
}

function setCORSHeaders(res: ServerResponse) {
  res.setHeader('Access-Control-Allow-Origin', allowedOrigin);
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  res.setHeader('Access-Control-Allow-Methods', 'GET,POST,PUT,PATCH,DELETE,OPTIONS');
}

function writeJSON(res: ServerResponse, status: number, payload: unknown) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(payload));
}

function writeRouteResult(res: ServerResponse, result: unknown) {
  if (isRouteHTTPResponse(result)) {
    writeJSON(res, result.__httpStatus, result.body);
    return;
  }
  writeJSON(res, 200, result);
}

function isRouteHTTPResponse(value: unknown): value is { __httpStatus: number; body: unknown } {
  return typeof value === 'object' &&
    value !== null &&
    typeof (value as { __httpStatus?: unknown }).__httpStatus === 'number' &&
    Object.prototype.hasOwnProperty.call(value, 'body');
}
`
}

func mobileBackendAuthAdapterTS() string {
	return `import type { IncomingHttpHeaders } from 'node:http';

export type MobileAuthUser = {
  id: string;
  email: string;
  name: string;
};

export type MobileAuthContext = {
  token: string;
  user: MobileAuthUser;
};

const demoToken = process.env.APEX_MOBILE_DEMO_TOKEN ?? 'dev-mobile-token';

export function issueDemoSession(email: string): MobileAuthContext {
  return {
    token: demoToken,
    user: {
      id: stableUserID(email),
      email,
      name: displayNameFromEmail(email)
    }
  };
}

export function authenticateMobileRequest(headers: IncomingHttpHeaders): MobileAuthContext | null {
  const raw = headers.authorization;
  const authorization = Array.isArray(raw) ? raw[0] ?? '' : raw ?? '';
  if (!authorization.startsWith('Bearer ')) return null;
  const token = authorization.slice('Bearer '.length).trim();
  if (!token) return null;
  return {
    token,
    user: {
      id: token === demoToken ? 'user-demo' : 'user-mobile',
      email: token === demoToken ? 'demo@apex-build.local' : 'mobile-user@apex-build.local',
      name: token === demoToken ? 'Demo Mobile User' : 'Mobile API User'
    }
  };
}

export function loginEmailFromBody(body: unknown): string {
  if (isRecord(body) && typeof body.email === 'string' && body.email.trim()) {
    return body.email.trim().toLowerCase();
  }
  return 'demo@apex-build.local';
}

function stableUserID(email: string) {
  return 'user-' + email.replace(/[^a-z0-9]+/gi, '-').replace(/^-+|-+$/g, '').toLowerCase();
}

function displayNameFromEmail(email: string) {
  const name = email.split('@')[0] ?? 'mobile user';
  return name.split(/[._-]+/).filter(Boolean).map((part) => part.slice(0, 1).toUpperCase() + part.slice(1)).join(' ') || 'Mobile User';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
`
}

func mobileBackendPersistenceAdapterTS(spec MobileAppSpec) string {
	demoDataJSON, _ := json.MarshalIndent(demoDataByType(spec), "", "  ")
	return fmt.Sprintf(`export type DataRecord = Record<string, unknown> & { id: string };

const seedData = %s satisfies Record<string, DataRecord[]>;

export class InMemoryMobileStore {
  private readonly collections = new Map<string, DataRecord[]>();

  constructor(initialData: Record<string, DataRecord[]>) {
    for (const [name, records] of Object.entries(initialData)) {
      this.collections.set(name, records.map((record) => ({ ...record })));
    }
  }

  list(typeName: string): DataRecord[] {
    return this.collection(typeName).map((record) => ({ ...record }));
  }

  first(typeName: string): DataRecord | undefined {
    const record = this.collection(typeName)[0];
    return record ? { ...record } : undefined;
  }

  get(typeName: string, id: string): DataRecord | undefined {
    const record = this.collection(typeName).find((candidate) => candidate.id === id);
    return record ? { ...record } : undefined;
  }

  upsert(typeName: string, record: DataRecord): DataRecord {
    const records = this.collection(typeName);
    const index = records.findIndex((candidate) => candidate.id === record.id);
    if (index >= 0) {
      records[index] = { ...records[index], ...record };
      return { ...records[index] };
    }
    records.push({ ...record });
    return { ...record };
  }

  append(typeName: string, record: DataRecord): DataRecord {
    this.collection(typeName).push({ ...record });
    return { ...record };
  }

  remove(typeName: string, id: string): DataRecord | undefined {
    const records = this.collection(typeName);
    const index = records.findIndex((candidate) => candidate.id === id);
    if (index < 0) return undefined;
    const [removed] = records.splice(index, 1);
    return removed ? { ...removed } : undefined;
  }

  private collection(typeName: string): DataRecord[] {
    const normalized = collectionName(typeName);
    const existing = this.collections.get(normalized);
    if (existing) return existing;
    const created: DataRecord[] = [];
    this.collections.set(normalized, created);
    return created;
  }
}

export const mobileStore = new InMemoryMobileStore(seedData);

export function collectionName(typeName: string) {
  return typeName.replace(/\[\]$/, '').trim() || 'Record';
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function numericField(source: Record<string, unknown>, key: string, fallback: number) {
  const value = source[key];
  return typeof value === 'number' ? value : fallback;
}
`, string(demoDataJSON))
}

func mobileBackendUploadAdapterTS() string {
	return `import { randomUUID } from 'node:crypto';
import type { IncomingHttpHeaders } from 'node:http';
import { mobileStore, type DataRecord } from './persistenceAdapter.js';

export type UploadMetadata = DataRecord & {
  url: string;
  contentType: string;
  createdAt: string;
  byteLength: number;
  ownerId?: string;
};

export function recordUpload(params: Record<string, string>, rawBody: Buffer, headers: IncomingHttpHeaders): UploadMetadata {
  const jobID = params.id ?? 'job-demo';
  const upload: UploadMetadata = {
    id: randomUUID(),
    jobId: jobID,
    url: '/uploads/' + encodeURIComponent(jobID) + '/' + randomUUID() + '.jpg',
    contentType: contentType(headers),
    createdAt: new Date().toISOString(),
    byteLength: rawBody.length
  };
  return mobileStore.append('PhotoAsset', upload) as UploadMetadata;
}

function contentType(headers: IncomingHttpHeaders) {
  const value = headers['content-type'];
  if (Array.isArray(value)) return value[0] ?? 'application/octet-stream';
  return value ?? 'application/octet-stream';
}
`
}

type backendContractDefinition struct {
	Name         string   `json:"name"`
	Helper       string   `json:"helper"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	RequestType  string   `json:"requestType"`
	ResponseType string   `json:"responseType"`
	BodyMode     string   `json:"bodyMode"`
	AuthRequired bool     `json:"authRequired"`
	PathParams   []string `json:"pathParams"`
}

func mobileBackendRoutesTS(spec MobileAppSpec) string {
	definitions := make([]backendContractDefinition, 0, len(spec.APIContracts))
	for _, endpoint := range apiEndpointDescriptors(spec.APIContracts) {
		bodyMode := "none"
		if endpoint.IsMultipart {
			bodyMode = "multipart"
		} else if endpoint.RequiresPayload {
			bodyMode = "json"
		}
		definitions = append(definitions, backendContractDefinition{
			Name:         endpoint.Name,
			Helper:       endpoint.FunctionName,
			Method:       endpoint.Method,
			Path:         endpoint.Path,
			RequestType:  endpoint.RequestType,
			ResponseType: endpoint.ResponseType,
			BodyMode:     bodyMode,
			AuthRequired: endpoint.AuthRequired,
			PathParams:   endpoint.PathParams,
		})
	}

	definitionsJSON, _ := json.MarshalIndent(definitions, "", "  ")
	return fmt.Sprintf(`import { randomUUID } from 'node:crypto';
import type { IncomingHttpHeaders } from 'node:http';
import { issueDemoSession, loginEmailFromBody, type MobileAuthContext } from './authAdapter.js';
import { collectionName, isRecord, mobileStore, numericField, type DataRecord } from './persistenceAdapter.js';
import { recordUpload } from './uploadAdapter.js';

type ContractDefinition = {
  name: string;
  helper: string;
  method: string;
  path: string;
  requestType: string;
  responseType: string;
  bodyMode: 'none' | 'json' | 'multipart';
  authRequired: boolean;
  pathParams: string[];
};

export type RouteContext = {
  params: Record<string, string>;
  body: unknown;
  rawBody: Buffer;
  headers: IncomingHttpHeaders;
  auth: MobileAuthContext | null;
};

export type ContractRoute = ContractDefinition & {
  match: (pathname: string) => Record<string, string> | null;
  handle: (ctx: RouteContext) => Promise<unknown>;
};

const contractDefinitions: ContractDefinition[] = %s;

export const mobileContractRoutes: ContractRoute[] = contractDefinitions.map((definition) => ({
  ...definition,
  match: createPathMatcher(definition.path),
  handle: async (ctx) => handleContractRoute(definition, ctx)
}));

async function handleContractRoute(definition: ContractDefinition, ctx: RouteContext): Promise<unknown> {
  switch (definition.responseType) {
    case 'AuthSession':
      return createAuthSession(ctx.body);
    case 'PhotoAsset':
      return createPhotoAsset(ctx);
    default:
      if (shouldPersistContractMutation(definition)) return persistContractMutation(definition, ctx);
      if (definition.method === 'DELETE') return deleteContractRecord(definition, ctx);
      return responseForType(definition.responseType, ctx);
  }
}

function createAuthSession(body: unknown) {
  return issueDemoSession(loginEmailFromBody(body));
}

function createPhotoAsset(ctx: RouteContext) {
  return recordUpload(ctx.params, ctx.rawBody, ctx.headers);
}

function shouldPersistContractMutation(definition: ContractDefinition) {
  const typeName = collectionName(definition.responseType);
  return ['POST', 'PUT', 'PATCH'].includes(definition.method) &&
    definition.bodyMode === 'json' &&
    typeName !== 'Record' &&
    typeName !== 'Unknown';
}

function persistContractMutation(definition: ContractDefinition, ctx: RouteContext) {
  const typeName = collectionName(definition.responseType);
  const record = buildRecordForMutation(typeName, ctx);
  return mobileStore.upsert(typeName, record);
}

function deleteContractRecord(definition: ContractDefinition, ctx: RouteContext) {
  const typeName = collectionName(definition.responseType);
  const id = firstPathParam(ctx.params) ?? '';
  if (!id) return routeHTTPResponse(400, { error: 'Missing record id', code: 'MISSING_RECORD_ID' });
  const removed = mobileStore.remove(typeName, id);
  if (!removed) return routeHTTPResponse(404, { error: 'Record not found', code: 'RECORD_NOT_FOUND', id, deleted: false });
  return { ...removed, deleted: true };
}

function buildRecordForMutation(typeName: string, ctx: RouteContext): DataRecord {
  const body = isRecord(ctx.body) ? ctx.body : {};
  const id = stringField(body, 'id') ?? firstPathParam(ctx.params) ?? randomUUID();
  const pathFields = Object.fromEntries(
    Object.entries(ctx.params).map(([key, value]) => [key === 'id' ? typeName.toLowerCase() + 'Id' : key, value])
  );
  const record: DataRecord = {
    ...pathFields,
    ...body,
    id,
    updatedAt: new Date().toISOString()
  };
  if (!record.createdAt) record.createdAt = record.updatedAt;
  return enrichRecordDefaults(typeName, record, ctx);
}

function enrichRecordDefaults(typeName: string, record: DataRecord, ctx: RouteContext): DataRecord {
  if (typeName !== 'Estimate') return record;
  return {
    ...record,
    jobId: stringField(record, 'jobId') ?? ctx.params.id ?? 'job-demo',
    laborHours: numericField(record, 'laborHours', 4),
    laborRate: numericField(record, 'laborRate', 85),
    materialsCost: numericField(record, 'materialsCost', 650),
    markupPercent: numericField(record, 'markupPercent', 18),
    finalPrice: numericField(record, 'finalPrice', 1170)
  };
}

function responseForType(typeName: string, ctx: RouteContext): unknown {
  if (typeName.endsWith('[]')) {
    const records = mobileStore.list(typeName);
    return records.length > 0 ? records : [fallbackObject(typeName.slice(0, -2), ctx)];
  }
  const id = firstPathParam(ctx.params);
  if (id) {
    const record = mobileStore.get(typeName, id);
    if (record) return record;
  }
  const demo = mobileStore.first(typeName);
  if (demo) return demo;
  return fallbackObject(typeName, ctx);
}

function fallbackObject(typeName: string, ctx: RouteContext) {
  return {
    id: randomUUID(),
    type: typeName || 'Unknown',
    params: ctx.params,
    received: isRecord(ctx.body) ? ctx.body : undefined,
    createdAt: new Date().toISOString()
  };
}

function firstPathParam(params: Record<string, string>) {
  return params.id ?? Object.values(params)[0];
}

function stringField(source: Record<string, unknown>, key: string) {
  const value = source[key];
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

function routeHTTPResponse(status: number, body: unknown) {
  return { __httpStatus: status, body };
}

export function createPathMatcher(pathTemplate: string) {
  const keys: string[] = [];
  const pattern = '^' + pathTemplate.split('/').map((part) => {
    if (part.startsWith(':')) {
      keys.push(part.slice(1));
      return '([^/]+)';
    }
    return escapeRegExp(part);
  }).join('/') + '/?$';
  const regex = new RegExp(pattern);
  return (pathname: string) => {
    const match = regex.exec(pathname);
    if (!match) return null;
    return keys.reduce<Record<string, string>>((params, key, index) => {
      params[key] = decodeURIComponent(match[index + 1] ?? '');
      return params;
    }, {});
  };
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
`, string(definitionsJSON))
}

func demoDataByType(spec MobileAppSpec) map[string][]map[string]any {
	demo := map[string][]map[string]any{}
	for _, model := range spec.DataModels {
		typeName := exportedTypeName(model.Name)
		if typeName == "" {
			continue
		}
		demo[typeName] = []map[string]any{sampleObjectForModel(typeName, model.Fields)}
	}
	if _, ok := demo["Customer"]; !ok {
		demo["Customer"] = []map[string]any{{
			"id":      "customer-demo",
			"name":    "Demo Customer",
			"phone":   "(555) 010-0101",
			"email":   "customer@example.test",
			"address": "100 Demo Way",
		}}
	}
	if _, ok := demo["Job"]; !ok {
		demo["Job"] = []map[string]any{{
			"id":             "job-demo",
			"title":          "Demo mobile job",
			"customerName":   "Demo Customer",
			"address":        "100 Demo Way",
			"status":         "Estimate needed",
			"estimatedValue": 1200,
			"syncState":      "synced",
		}}
	}
	if _, ok := demo["Estimate"]; !ok {
		demo["Estimate"] = []map[string]any{{
			"id":            "estimate-demo",
			"jobId":         "job-demo",
			"laborHours":    4,
			"laborRate":     85,
			"materialsCost": 650,
			"markupPercent": 18,
			"finalPrice":    1170,
		}}
	}
	return demo
}

func sampleObjectForModel(typeName string, fields map[string]string) map[string]any {
	sample := map[string]any{}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		sample[tsPropertyName(key)] = sampleValueForField(tsPropertyName(key), fields[key])
	}
	if _, ok := sample["id"]; !ok {
		sample["id"] = strings.ToLower(typeName) + "-demo"
	}
	if typeName == "Job" {
		if _, ok := sample["customerName"]; !ok {
			sample["customerName"] = "Demo Customer"
		}
		if _, ok := sample["address"]; !ok {
			sample["address"] = "100 Demo Way"
		}
		if _, ok := sample["estimatedValue"]; !ok {
			sample["estimatedValue"] = 1200
		}
		if _, ok := sample["syncState"]; !ok {
			sample["syncState"] = "synced"
		}
	}
	return sample
}

func sampleValueForField(name string, fieldType string) any {
	switch strings.ToLower(strings.TrimSpace(fieldType)) {
	case "number", "integer", "int", "float", "decimal":
		return 1
	case "boolean":
		return true
	case "date", "datetime", "time":
		return "2026-01-01T00:00:00Z"
	default:
		switch strings.ToLower(name) {
		case "email":
			return "demo@example.test"
		case "phone":
			return "(555) 010-0101"
		case "status":
			return "active"
		case "title":
			return "Demo " + name
		default:
			return name + "-demo"
		}
	}
}

func MobileBackendRoutesMarkdown(spec MobileAppSpec) string {
	var b strings.Builder
	b.WriteString("# Generated Mobile Backend Routes\n\n")
	b.WriteString("These routes are generated from `MobileAppSpec.APIContracts`, the same source used for `mobile/docs/api-contract.json` and `mobile/src/api/endpoints.ts`.\n\n")
	b.WriteString("| Helper | Method | Path | Auth | Body | Response |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, endpoint := range apiEndpointDescriptors(spec.APIContracts) {
		bodyMode := "none"
		if endpoint.IsMultipart {
			bodyMode = "multipart"
		} else if endpoint.RequiresPayload {
			bodyMode = "json"
		}
		auth := "yes"
		if !endpoint.AuthRequired {
			auth = "no"
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %s | %s | `%s` |\n", endpoint.FunctionName, endpoint.Method, endpoint.Path, auth, bodyMode, endpoint.ResponseType))
	}
	b.WriteString("\nThe generated backend is a runnable local starter with in-memory/demo responses. Replace it with production auth, persistence, storage, and notification infrastructure before release.\n")
	return b.String()
}
