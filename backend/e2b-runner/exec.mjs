import { CommandExitError, Sandbox } from 'e2b/dist/index.mjs'

async function readRequest() {
  const chunks = []
  for await (const chunk of process.stdin) {
    chunks.push(typeof chunk === 'string' ? Buffer.from(chunk) : chunk)
  }

  const raw = Buffer.concat(chunks).toString('utf8').trim()
  if (!raw) {
    throw new Error('missing helper request payload')
  }

  return JSON.parse(raw)
}

function requireApiKey() {
  const apiKey = process.env.E2B_API_KEY?.trim()
  if (!apiKey) {
    throw new Error('E2B_API_KEY is not set')
  }
  return apiKey
}

function respond(payload) {
  process.stdout.write(`${JSON.stringify(payload)}\n`)
}

async function createSandbox(apiKey, request) {
  const sandbox = await Sandbox.create('base', {
    apiKey,
    timeoutMs: request.timeoutMs ?? 30_000,
    secure: true,
    allowInternetAccess: true,
    requestTimeoutMs: Math.max((request.timeoutMs ?? 30_000) + 10_000, 60_000),
  })

  respond({ sandboxId: sandbox.sandboxId })
}

async function writeFile(apiKey, request) {
  if (!request.sandboxId) {
    throw new Error('sandboxId is required for write')
  }
  if (!request.path) {
    throw new Error('path is required for write')
  }

  const sandbox = await Sandbox.connect(request.sandboxId, {
    apiKey,
    requestTimeoutMs: 60_000,
  })
  await sandbox.files.write(request.path, request.content ?? '')
  respond({ ok: true })
}

async function runCommand(apiKey, request) {
  if (!request.sandboxId) {
    throw new Error('sandboxId is required for run')
  }
  if (!request.command) {
    throw new Error('command is required for run')
  }

  const sandbox = await Sandbox.connect(request.sandboxId, {
    apiKey,
    requestTimeoutMs: Math.max((request.timeoutMs ?? 30_000) + 10_000, 60_000),
  })

  try {
    const result = await sandbox.commands.run(request.command, {
      cwd: request.cwd,
      envs: request.env,
      timeoutMs: request.timeoutMs ?? 30_000,
      requestTimeoutMs: Math.max((request.timeoutMs ?? 30_000) + 10_000, 60_000),
    })
    respond({
      exitCode: result.exitCode,
      stdout: result.stdout,
      stderr: result.stderr,
    })
  } catch (error) {
    if (error instanceof CommandExitError) {
      respond({
        exitCode: error.exitCode,
        stdout: error.stdout,
        stderr: error.stderr,
      })
      return
    }
    throw error
  }
}

async function startCommand(apiKey, request) {
  if (!request.sandboxId) {
    throw new Error('sandboxId is required for start')
  }
  if (!request.command) {
    throw new Error('command is required for start')
  }

  const sandbox = await Sandbox.connect(request.sandboxId, {
    apiKey,
    requestTimeoutMs: Math.max((request.timeoutMs ?? 30_000) + 10_000, 60_000),
  })

  const handle = await sandbox.commands.start(request.command, {
    cwd: request.cwd,
    envs: request.env,
    timeoutMs: request.timeoutMs ?? 30_000,
    requestTimeoutMs: Math.max((request.timeoutMs ?? 30_000) + 10_000, 60_000),
  })

  const host = request.port ? sandbox.getHost(request.port) : ''
  respond({
    pid: handle.pid,
    host,
    url: host ? `https://${host}` : '',
  })
}

async function waitForCommand(apiKey, request) {
  if (!request.sandboxId) {
    throw new Error('sandboxId is required for wait')
  }
  if (!request.pid) {
    throw new Error('pid is required for wait')
  }

  const sandbox = await Sandbox.connect(request.sandboxId, {
    apiKey,
    requestTimeoutMs: Math.max((request.timeoutMs ?? 30_000) + 10_000, 60_000),
  })

  try {
    const handle = await sandbox.commands.connect(request.pid, {
      timeoutMs: request.timeoutMs ?? 30_000,
      requestTimeoutMs: Math.max((request.timeoutMs ?? 30_000) + 10_000, 60_000),
    })
    const result = await handle.wait()
    respond({
      exitCode: result.exitCode,
      stdout: result.stdout,
      stderr: result.stderr,
    })
  } catch (error) {
    if (error instanceof CommandExitError) {
      respond({
        exitCode: error.exitCode,
        stdout: error.stdout,
        stderr: error.stderr,
      })
      return
    }
    throw error
  }
}

async function killProcess(apiKey, request) {
  if (!request.sandboxId) {
    throw new Error('sandboxId is required for kill_process')
  }
  if (!request.pid) {
    throw new Error('pid is required for kill_process')
  }

  const sandbox = await Sandbox.connect(request.sandboxId, {
    apiKey,
    requestTimeoutMs: 60_000,
  })
  const killed = await sandbox.commands.kill(request.pid, {
    requestTimeoutMs: 60_000,
  })
  respond({ killed })
}

async function killSandbox(apiKey, request) {
  if (!request.sandboxId) {
    throw new Error('sandboxId is required for kill')
  }

  const killed = await Sandbox.kill(request.sandboxId, {
    apiKey,
    requestTimeoutMs: 60_000,
  })
  respond({ killed })
}

async function main() {
  const request = await readRequest()
  const apiKey = requireApiKey()

  switch (request.action) {
    case 'create':
      await createSandbox(apiKey, request)
      break
    case 'write':
      await writeFile(apiKey, request)
      break
    case 'run':
      await runCommand(apiKey, request)
      break
    case 'start':
      await startCommand(apiKey, request)
      break
    case 'wait':
      await waitForCommand(apiKey, request)
      break
    case 'kill_process':
      await killProcess(apiKey, request)
      break
    case 'kill':
      await killSandbox(apiKey, request)
      break
    default:
      throw new Error(`unsupported action: ${request.action ?? '<missing>'}`)
  }
}

main().catch((error) => {
  const message =
    error instanceof Error
      ? error.stack || error.message
      : typeof error === 'string'
        ? error
        : JSON.stringify(error)
  process.stderr.write(`${message}\n`)
  process.exit(1)
})
