import cors from 'cors';
import crypto from 'crypto';
import dotenv from 'dotenv';
import express from 'express';
import fs from 'fs/promises';
import path from 'path';
import { Client } from 'ssh2';
import { fileURLToPath } from 'url';

dotenv.config({ quiet: true });

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();
const HOST = process.env.HOST || '127.0.0.1';
const PORT = Number(process.env.PORT || 3001);
const SESSION_TTL_MS = 1000 * 60 * 60 * 12;
const DATA_DIR = path.join(__dirname, 'data');
const AUTH_FILE = path.join(DATA_DIR, 'panel-auth.json');
const sessions = new Map();

function getAllowedOrigins() {
  const configured = process.env.ALLOWED_ORIGINS
    ?.split(',')
    .map((origin) => origin.trim())
    .filter(Boolean);

  return configured?.length ? configured : ['http://localhost:5173', 'http://localhost:3000'];
}

app.use(
  cors({
    origin(origin, callback) {
      if (!origin || getAllowedOrigins().includes(origin)) {
        callback(null, true);
        return;
      }

      callback(new Error('Origin is not allowed by CORS'));
    },
  }),
);
app.use(express.json({ limit: '2mb' }));

app.use((req, res, next) => {
  console.log(`${new Date().toISOString()} ${req.method} ${req.path}`);
  next();
});

function hashPassword(password, salt = crypto.randomBytes(16).toString('hex')) {
  const hash = crypto.scryptSync(password, salt, 64).toString('hex');
  return `${salt}:${hash}`;
}

function verifyPassword(password, storedHash) {
  const [salt, hash] = String(storedHash).split(':');
  if (!salt || !hash) {
    return false;
  }

  const derived = crypto.scryptSync(password, salt, 64);
  const stored = Buffer.from(hash, 'hex');

  if (derived.length !== stored.length) {
    return false;
  }

  return crypto.timingSafeEqual(derived, stored);
}

async function readJson(filePath, fallback) {
  try {
    const contents = await fs.readFile(filePath, 'utf8');
    return JSON.parse(contents);
  } catch (error) {
    if (error.code === 'ENOENT') {
      return fallback;
    }

    throw error;
  }
}

async function writeJson(filePath, value) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, `${JSON.stringify(value, null, 2)}\n`, 'utf8');
}

async function ensureAuthStore() {
  const existing = await readJson(AUTH_FILE, null);
  if (existing) {
    return existing;
  }

  const username = process.env.PANEL_ADMIN_USERNAME || 'admin';
  const password = process.env.PANEL_ADMIN_PASSWORD || 'admin123!';
  const defaultCredentials =
    !process.env.PANEL_ADMIN_USERNAME || !process.env.PANEL_ADMIN_PASSWORD;

  const authStore = {
    username,
    passwordHash: hashPassword(password),
    defaultCredentials,
    createdAt: new Date().toISOString(),
    passwordUpdatedAt: new Date().toISOString(),
  };

  await writeJson(AUTH_FILE, authStore);

  if (defaultCredentials) {
    console.log(
      'Panel auth uses default credentials: admin / admin123! . Change the password after login.',
    );
  }

  return authStore;
}

function createSession(username) {
  const token = crypto.randomBytes(32).toString('hex');
  const expiresAt = Date.now() + SESSION_TTL_MS;
  sessions.set(token, { username, expiresAt });
  return { token, expiresAt };
}

function getSessionFromRequest(req) {
  const authorization = req.get('authorization') || '';
  if (!authorization.startsWith('Bearer ')) {
    return null;
  }

  const token = authorization.slice(7).trim();
  const session = sessions.get(token);

  if (!session) {
    return null;
  }

  if (session.expiresAt <= Date.now()) {
    sessions.delete(token);
    return null;
  }

  return { token, ...session };
}

async function authMiddleware(req, res, next) {
  const session = getSessionFromRequest(req);
  if (!session) {
    res.status(401).json({ error: 'Authentication required' });
    return;
  }

  const authStore = await ensureAuthStore();
  req.auth = {
    token: session.token,
    username: session.username,
    expiresAt: session.expiresAt,
    defaultCredentials: Boolean(authStore.defaultCredentials),
    passwordUpdatedAt: authStore.passwordUpdatedAt,
  };

  next();
}

function shellEscape(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

function normalizeServerConfig(rawConfig) {
  const config = rawConfig || {};
  const normalized = {
    host: String(config.host || '').trim(),
    port: Number(config.port) || 22,
    username: String(config.username || '').trim(),
    password: String(config.password || '').trim(),
    privateKey: String(config.privateKey || '').trim(),
  };

  if (!normalized.host || !normalized.username) {
    throw new Error('Host and username are required');
  }

  if (!normalized.password && !normalized.privateKey) {
    throw new Error('Password or private key is required');
  }

  return normalized;
}

function connectSSH(serverConfig) {
  return new Promise((resolve, reject) => {
    const client = new Client();
    let settled = false;

    client
      .on('ready', () => {
        settled = true;
        resolve(client);
      })
      .on('error', (error) => {
        if (!settled) {
          reject(error);
        }
      });

    const connectionOptions = {
      host: serverConfig.host,
      port: serverConfig.port,
      username: serverConfig.username,
      readyTimeout: 30000,
    };

    if (serverConfig.privateKey) {
      connectionOptions.privateKey = serverConfig.privateKey;
    } else {
      connectionOptions.password = serverConfig.password;
    }

    client.connect(connectionOptions);
  });
}

function execSSH(client, command) {
  return new Promise((resolve, reject) => {
    client.exec(command, (error, stream) => {
      if (error) {
        reject(error);
        return;
      }

      let stdout = '';
      let stderr = '';

      stream.on('close', (code) => {
        resolve({
          code: typeof code === 'number' ? code : 0,
          stdout,
          stderr,
        });
      });

      stream.on('data', (chunk) => {
        stdout += chunk.toString();
      });

      stream.stderr.on('data', (chunk) => {
        stderr += chunk.toString();
      });
    });
  });
}

async function withSSH(serverConfig, script) {
  const client = await connectSSH(normalizeServerConfig(serverConfig));

  try {
    const result = await execSSH(client, `bash -lc ${shellEscape(script)}`);
    if (result.code !== 0) {
      throw new Error(result.stderr.trim() || 'SSH command failed');
    }

    return result.stdout;
  } finally {
    client.end();
  }
}

function parseSystemInfo(stdout) {
  const [hostname, uptime, cpuCores, cpuModel, memory, disk, os, kernel] = stdout
    .split('\n')
    .map((line) => line.trim());

  const [memoryTotal = '0', memoryUsed = '0'] = memory.split(':');
  const [diskSize = '0', diskUsed = '0', diskAvailable = '0', diskPercentRaw = '0'] =
    disk.split(':');
  const diskPercent = Number(String(diskPercentRaw).replace('%', '')) || 0;

  return {
    hostname,
    uptime,
    cpuCores: Number(cpuCores) || 0,
    cpuModel,
    memoryTotal: Number(memoryTotal) || 0,
    memoryUsed: Number(memoryUsed) || 0,
    memoryPercent:
      Number(memoryTotal) > 0
        ? Math.round((Number(memoryUsed) / Number(memoryTotal)) * 100)
        : 0,
    diskSize,
    diskUsed,
    diskAvailable,
    diskPercent,
    os,
    kernel,
  };
}

function parseMetrics(stdout) {
  const [cpu, memory, disk, loadAverage] = stdout
    .split('\n')
    .map((line) => Number(line.trim()) || 0);

  return {
    timestamp: Date.now(),
    cpu,
    memory,
    disk,
    loadAverage,
  };
}

function parseFiles(stdout) {
  return stdout
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [type, size, modified, permissions, ...nameParts] = line.split('\t');
      return {
        name: nameParts.join('\t'),
        size: Number(size) || 0,
        modified,
        permissions,
        type,
        isDirectory: type === 'd',
      };
    });
}

function parseLoginLogs(stdout) {
  return stdout
    .split('\n')
    .map((line) => line.replace(/\s+/g, ' ').trim())
    .filter((line) => line && !line.includes('wtmp begins'))
    .slice(0, 50)
    .map((line, index) => {
      const parts = line.split(' ');
      const user = parts[0] || 'unknown';
      const terminal = parts[1] || '-';
      const ip = parts[2] || '-';
      const time = parts.slice(3).join(' ');
      const status = line.includes('still logged in') ? 'active' : 'success';

      return {
        id: index + 1,
        user,
        terminal,
        ip,
        time,
        status,
        raw: line,
      };
    });
}

app.get('/api/health', async (req, res) => {
  const authStore = await ensureAuthStore();

  res.json({
    status: 'ok',
    timestamp: new Date().toISOString(),
    auth: {
      username: authStore.username,
      defaultCredentials: Boolean(authStore.defaultCredentials),
    },
  });
});

app.get('/api/auth/status', async (req, res) => {
  const authStore = await ensureAuthStore();
  res.json({
    username: authStore.username,
    defaultCredentials: Boolean(authStore.defaultCredentials),
  });
});

app.post('/api/auth/login', async (req, res) => {
  const { username, password } = req.body || {};
  const authStore = await ensureAuthStore();

  if (username !== authStore.username || !verifyPassword(String(password || ''), authStore.passwordHash)) {
    res.status(401).json({ error: 'Invalid username or password' });
    return;
  }

  const session = createSession(authStore.username);

  res.json({
    success: true,
    token: session.token,
    expiresAt: session.expiresAt,
    user: {
      username: authStore.username,
      defaultCredentials: Boolean(authStore.defaultCredentials),
      passwordUpdatedAt: authStore.passwordUpdatedAt,
    },
  });
});

app.use('/api', authMiddleware);

app.get('/api/auth/me', (req, res) => {
  res.json({
    user: {
      username: req.auth.username,
      defaultCredentials: req.auth.defaultCredentials,
      passwordUpdatedAt: req.auth.passwordUpdatedAt,
    },
    expiresAt: req.auth.expiresAt,
  });
});

app.post('/api/auth/logout', (req, res) => {
  sessions.delete(req.auth.token);
  res.json({ success: true });
});

app.post('/api/auth/change-password', async (req, res) => {
  const { currentPassword, newPassword } = req.body || {};
  const authStore = await ensureAuthStore();

  if (!verifyPassword(String(currentPassword || ''), authStore.passwordHash)) {
    res.status(400).json({ error: 'Current password is incorrect' });
    return;
  }

  if (typeof newPassword !== 'string' || newPassword.trim().length < 8) {
    res.status(400).json({ error: 'New password must contain at least 8 characters' });
    return;
  }

  const updatedStore = {
    ...authStore,
    passwordHash: hashPassword(newPassword.trim()),
    defaultCredentials: false,
    passwordUpdatedAt: new Date().toISOString(),
  };

  await writeJson(AUTH_FILE, updatedStore);
  sessions.clear();

  res.json({
    success: true,
    message: 'Password changed. Sign in again with the new password.',
  });
});

app.post('/api/test-connection', async (req, res) => {
  try {
    const stdout = await withSSH(
      req.body,
      "hostname && printf '\\n' && uname -sr",
    );

    res.json({
      success: true,
      message: 'SSH connection established successfully',
      systemInfo: stdout.trim(),
    });
  } catch (error) {
    res.status(400).json({
      success: false,
      message: error.message,
    });
  }
});

app.post('/api/system-info', async (req, res) => {
  try {
    const stdout = await withSSH(
      req.body.config || req.body.serverConfig || req.body,
      [
        'HOSTNAME=$(hostname)',
        "UPTIME=$(uptime -p | sed 's/^up //')",
        'CPU_CORES=$(nproc)',
        "CPU_MODEL=$(lscpu | awk -F: '/Model name/{gsub(/^[ \\t]+/, \"\", $2); print $2; exit}')",
        "MEMORY=$(free -m | awk '/Mem:/{print $2\":\"$3}')",
        "DISK=$(df -h / | awk 'END{print $2\":\"$3\":\"$4\":\"$5}')",
        "OS=$(grep '^PRETTY_NAME=' /etc/os-release | cut -d= -f2- | tr -d '\"')",
        'KERNEL=$(uname -r)',
        'printf "%s\\n%s\\n%s\\n%s\\n%s\\n%s\\n%s\\n%s\\n" "$HOSTNAME" "$UPTIME" "$CPU_CORES" "$CPU_MODEL" "$MEMORY" "$DISK" "$OS" "$KERNEL"',
      ].join('\n'),
    );

    res.json(parseSystemInfo(stdout));
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/metrics', async (req, res) => {
  try {
    const stdout = await withSSH(
      req.body.config || req.body.serverConfig || req.body,
      [
        "CPU=$(top -bn1 | awk -F',' '/Cpu\\(s\\)/{for(i=1;i<=NF;i++){if($i ~ /id/){gsub(/[^0-9.]/,\"\",$i); printf \"%.1f\", 100 - $i; break}}}')",
        "MEMORY=$(free | awk '/Mem:/{printf \"%.1f\", ($3/$2)*100}')",
        "DISK=$(df / | awk 'END{gsub(/%/, \"\", $5); print $5}')",
        "LOAD=$(cut -d' ' -f1 /proc/loadavg)",
        'printf "%s\\n%s\\n%s\\n%s\\n" "$CPU" "$MEMORY" "$DISK" "$LOAD"',
      ].join('\n'),
    );

    res.json(parseMetrics(stdout));
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/files/list', async (req, res) => {
  try {
    const targetPath = String(req.body.path || '/');
    const stdout = await withSSH(
      req.body.config || req.body.serverConfig || req.body,
      `find ${shellEscape(targetPath)} -mindepth 1 -maxdepth 1 -printf '%y\t%s\t%TY-%Tm-%Td %TH:%TM\t%M\t%f\n' | sort`,
    );

    res.json({
      path: targetPath,
      files: parseFiles(stdout),
    });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/files/read', async (req, res) => {
  try {
    const targetPath = String(req.body.path || '');
    if (!targetPath) {
      throw new Error('Path is required');
    }

    const stdout = await withSSH(
      req.body.config || req.body.serverConfig || req.body,
      `cat ${shellEscape(targetPath)}`,
    );

    res.json({ content: stdout });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/files/write', async (req, res) => {
  try {
    const targetPath = String(req.body.path || '');
    if (!targetPath) {
      throw new Error('Path is required');
    }

    const encodedContent = Buffer.from(String(req.body.content || ''), 'utf8').toString('base64');
    await withSSH(
      req.body.config || req.body.serverConfig || req.body,
      `printf '%s' ${shellEscape(encodedContent)} | base64 -d > ${shellEscape(targetPath)}`,
    );

    res.json({ success: true });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/files/delete', async (req, res) => {
  try {
    const targetPath = String(req.body.path || '');
    const isDirectory = Boolean(req.body.isDirectory);

    if (!targetPath) {
      throw new Error('Path is required');
    }

    await withSSH(
      req.body.config || req.body.serverConfig || req.body,
      `${isDirectory ? 'rm -rf' : 'rm -f'} -- ${shellEscape(targetPath)}`,
    );

    res.json({ success: true });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/logs/auth', async (req, res) => {
  try {
    const stdout = await withSSH(
      req.body.config || req.body.serverConfig || req.body,
      'last -n 50 -i',
    );

    res.json({
      logs: parseLoginLogs(stdout),
    });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/software/install', async (req, res) => {
  try {
    const software = String(req.body.software || '').trim();
    if (!software) {
      throw new Error('Software id is required');
    }

    const commands = {
      docker: 'curl -fsSL https://get.docker.com | sh',
      nodejs: 'apt-get update && apt-get install -y nodejs npm',
      nginx: 'apt-get update && apt-get install -y nginx',
      postgresql: 'apt-get update && apt-get install -y postgresql',
      redis: 'apt-get update && apt-get install -y redis-server',
      fail2ban: 'apt-get update && apt-get install -y fail2ban',
      certbot: 'apt-get update && apt-get install -y certbot',
    };

    const command = commands[software] || `apt-get update && apt-get install -y ${software}`;
    const stdout = await withSSH(req.body.config || req.body.serverConfig || req.body, command);

    res.json({
      success: true,
      output: stdout.trim(),
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      error: error.message,
    });
  }
});

const sessionCleanup = setInterval(() => {
  const now = Date.now();
  for (const [token, session] of sessions.entries()) {
    if (session.expiresAt <= now) {
      sessions.delete(token);
    }
  }
}, 1000 * 60 * 5);

sessionCleanup.unref();

await ensureAuthStore();

app.listen(PORT, HOST, () => {
  console.log(`VDS Panel backend started on http://${HOST}:${PORT}`);
});

process.on('SIGTERM', () => {
  clearInterval(sessionCleanup);
  process.exit(0);
});

process.on('SIGINT', () => {
  clearInterval(sessionCleanup);
  process.exit(0);
});
