const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

const candidateTerms = [
  'forecast',
  'rain',
  'radar',
  'map',
  'overlay',
  'product',
  'point',
  'meteogram',
  'picker',
];

function parseArgs(argv) {
  const opts = {
    url: 'https://www.windy.com/-Rain-thunder-rain?rain,52.469,13.372,10,p:cities,m:e6Fagxs',
    lat: 52.469,
    lon: 13.372,
    screenshotPath: '',
    debugNetwork: false,
    timeoutMs: 45000,
    headless: true,
  };

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '--url' && argv[i + 1]) {
      opts.url = argv[++i];
    } else if (arg === '--lat' && argv[i + 1]) {
      opts.lat = Number(argv[++i]);
    } else if (arg === '--lon' && argv[i + 1]) {
      opts.lon = Number(argv[++i]);
    } else if (arg === '--screenshot' && argv[i + 1]) {
      opts.screenshotPath = argv[++i];
    } else if (arg === '--debug-network') {
      opts.debugNetwork = true;
    } else if (arg === '--timeout' && argv[i + 1]) {
      opts.timeoutMs = Number.parseInt(argv[++i], 10);
    } else if (arg === '--non-headless') {
      opts.headless = false;
    }
  }
  return opts;
}

function sanitizeUrl(rawUrl) {
  try {
    const parsed = new URL(rawUrl);
    const sensitive = ['token', 'token2', 'auth', 'authorization', 'key', 'api_key', 'apikey', 'cookie', 'sid', 'session', 'uid', 'uh', 'cid'];
    for (const key of Array.from(parsed.searchParams.keys())) {
      const lower = key.toLowerCase();
      if (sensitive.some((term) => lower.includes(term))) {
        parsed.searchParams.delete(key);
      }
    }
    return parsed.toString();
  } catch {
    return rawUrl;
  }
}

function shouldCapture(rawUrl, contentType = '') {
  const url = rawUrl.toLowerCase();
  if (url.includes('/sedlina/ga/')) {
    return false;
  }
  if (url.includes('forecast/point') || url.includes('metadata/v1.0/forecast')) {
    return true;
  }
  return candidateTerms.some((term) => url.includes(term));
}

function preview(text, max = 400) {
  if (!text) {
    return '';
  }
  return text.slice(0, max).replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/g, '');
}

function byteLength(text) {
  return Buffer.byteLength(text || '', 'utf8');
}

async function responseRecord(response) {
  const rawUrl = response.url();
  const contentType = response.headers()['content-type'] || '';
  if (!shouldCapture(rawUrl, contentType)) {
    return null;
  }

  const record = {
    url: sanitizeUrl(rawUrl),
    method: response.request().method(),
    status: response.status(),
    contentType,
    size: 0,
  };

  if (response.status() >= 200 && response.status() < 300) {
    const lowerType = contentType.toLowerCase();
    const isText = lowerType.includes('json') || lowerType.includes('text/') || lowerType.includes('javascript');
    if (isText) {
      try {
        const body = await response.text();
        record.size = byteLength(body);
        record.preview = preview(body);
        if (lowerType.includes('json')) {
          record.body = body;
        }
      } catch {
        record.preview = '';
      }
    } else {
      const length = response.headers()['content-length'];
      record.size = length ? Number.parseInt(length, 10) || 0 : 0;
    }
  }

  return record;
}

function refTimeParam(ref) {
  const date = new Date(ref);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  const pad = (n) => String(n).padStart(2, '0');
  return `${date.getUTCFullYear()}${pad(date.getUTCMonth() + 1)}${pad(date.getUTCDate())}${pad(date.getUTCHours())}`;
}

async function fetchPublicPointData(page, lat, lon) {
    return page.evaluate(async ({ lat, lon }) => {
      const out = { responses: [], endpoints: [], error: '' };
      const refTimeParam = (ref) => {
        const date = new Date(ref);
        if (Number.isNaN(date.getTime())) {
          return '';
        }
        const pad = (n) => String(n).padStart(2, '0');
        return `${date.getUTCFullYear()}${pad(date.getUTCMonth() + 1)}${pad(date.getUTCDate())}${pad(date.getUTCHours())}`;
      };
      const readJsonText = async (url) => {
        const response = await fetch(url, {
          credentials: 'omit',
          headers: { Accept: 'application/json' },
        });
        const text = await response.text();
        const record = {
          url,
          method: 'GET',
          status: response.status,
          contentType: response.headers.get('content-type') || '',
          size: new TextEncoder().encode(text).length,
          preview: text.slice(0, 400),
          body: text,
        };
        out.responses.push(record);
        out.endpoints.push(url);
        if (!response.ok) {
          throw new Error(`Windy public endpoint returned ${response.status}: ${url}`);
        }
        return JSON.parse(text);
      };

      try {
        const minifestUrl = 'https://node.windy.com/metadata/v1.0/forecast/ecmwf-hres/minifest.json';
        const minifest = await readJsonText(minifestUrl);
        const refTime = refTimeParam(minifest.ref);
        if (!refTime) {
          throw new Error('Windy minifest did not contain a parseable ref time');
        }

        const latText = Number(lat).toFixed(3);
        const lonText = Number(lon).toFixed(3);
        await readJsonText(`https://node.windy.com/forecast/point/now/ecmwf/v1.0/${latText}/${lonText}?refTime=${refTime}`);
        await readJsonText(`https://node.windy.com/forecast/point/ecmwf/v2.9/${latText}/${lonText}?refTime=${refTime}&step=3&interpolate=true`);

        // Fetch GFS and ICON for multi-model agreement
        const models = ['gfs', 'icon'];
        for (const model of models) {
          try {
            await readJsonText(`https://node.windy.com/forecast/point/${model}/v2.9/${latText}/${lonText}?refTime=${refTime}&step=3&interpolate=true`);
          } catch {
            // Non-critical: continue if secondary model fails
          }
        }
      } catch (error) {
        out.error = error.message;
      }
      return out;
    }, { lat, lon });
  }

function mergeRecords(existing, incoming) {
  const seen = new Set(existing.map((record) => `${record.method} ${record.url}`));
  for (const record of incoming) {
    record.url = sanitizeUrl(record.url);
    if (!record.preview && record.body) {
      record.preview = preview(record.body);
    }
    const key = `${record.method} ${record.url}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    existing.push(record);
  }
}

async function run() {
  const opts = parseArgs(process.argv.slice(2));
  const interceptedResponses = [];
  const endpointsUsed = [];
  let browser;

  try {
    browser = await chromium.launch({
      headless: opts.headless,
      args: ['--no-sandbox', '--disable-setuid-sandbox'],
    });

    const context = await browser.newContext({
      viewport: { width: 1280, height: 720 },
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    });

    const page = await context.newPage();
    page.on('response', async (response) => {
      try {
        const record = await responseRecord(response);
        if (record) {
          interceptedResponses.push(record);
        }
      } catch {
        // Ignore detached response bodies.
      }
    });

    await page.goto(opts.url, { waitUntil: 'domcontentloaded', timeout: opts.timeoutMs });
    await page.waitForSelector('#leaflet-map', { timeout: opts.timeoutMs });
    await page.waitForTimeout(3500);
    await page.mouse.click(640, 360);
    await page.waitForTimeout(1500);

    const pointData = await fetchPublicPointData(page, opts.lat, opts.lon);
    mergeRecords(interceptedResponses, pointData.responses);
    endpointsUsed.push(...pointData.endpoints.map(sanitizeUrl));
    if (pointData.error) {
      interceptedResponses.push({
        url: 'https://node.windy.com/forecast/point',
        method: 'GET',
        status: 0,
        contentType: 'application/json',
        size: 0,
        preview: pointData.error,
      });
    }

    let screenshotSaved = false;
    let screenshotPath = '';
    if (opts.screenshotPath) {
      screenshotPath = path.resolve(opts.screenshotPath);
      fs.mkdirSync(path.dirname(screenshotPath), { recursive: true });
      await page.screenshot({ path: screenshotPath, fullPage: false });
      screenshotSaved = true;
    }

    let debugLogSaved = false;
    let debugLogPath = '';
    if (opts.debugNetwork) {
      debugLogPath = path.join(process.cwd(), 'debug', 'windy-network-log.json');
      fs.mkdirSync(path.dirname(debugLogPath), { recursive: true });
      const formattedLog = interceptedResponses.map((record) => ({
        url: record.url,
        method: record.method,
        status: record.status,
        contentType: record.contentType,
        size: record.size,
        preview: record.preview || preview(record.body || ''),
      }));
      fs.writeFileSync(debugLogPath, JSON.stringify(formattedLog, null, 2), 'utf8');
      debugLogSaved = true;
    }

    console.log(JSON.stringify({
      success: true,
      responses: interceptedResponses,
      endpoints_used: Array.from(new Set(endpointsUsed)),
      screenshot_saved: screenshotSaved,
      screenshot_path: screenshotPath,
      debug_log_saved: debugLogSaved,
      debug_log_path: debugLogPath,
    }));
  } catch (error) {
    console.log(JSON.stringify({
      success: false,
      error: error.message,
      responses: interceptedResponses,
      endpoints_used: Array.from(new Set(endpointsUsed)),
    }));
  } finally {
    if (browser) {
      await browser.close();
    }
  }
}

run();
