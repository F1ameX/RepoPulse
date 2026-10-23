// @vitest-environment node
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, it } from 'vitest';
import api, { ApiError } from './api';

const server = createServer((request, response) => {
  if (request.url === '/reset') {
    request.socket.destroy();
    return;
  }
  if (request.url === '/timeout') return;
  if (request.url === '/proxy-error') {
    response.writeHead(502, { 'Content-Type': 'text/html' });
    response.end('<h1>Bad gateway</h1>');
    return;
  }
  if (request.url === '/echo') {
    let body = '';
    request.on('data', (chunk) => {
      body += chunk;
    });
    request.on('end', () => {
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify({ method: request.method, body: JSON.parse(body) }));
    });
    return;
  }
  const code =
    request.url === '/private' ? 'REPOSITORY_NOT_FOUND_OR_PRIVATE' : 'UPSTREAM_RATE_LIMITED';
  response.writeHead(request.url === '/private' ? 404 : 503, {
    'Content-Type': 'application/json',
  });
  response.end(JSON.stringify({ error: { code, message: 'Controlled test response' } }));
});
let origin: string;

beforeAll(async () => {
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});
afterAll(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
});

it.each([
  ['/private', 404, 'REPOSITORY_NOT_FOUND_OR_PRIVATE'],
  ['/rate-limit', 503, 'UPSTREAM_RATE_LIMITED'],
  ['/proxy-error', 502, 'INTERNAL_ERROR'],
] as const)('preserves stable codes for HTTP response %s', async (path, status, code) => {
  await expect(api.get(origin + path)).rejects.toMatchObject({ name: 'ApiError', code, status });
});

it.each(['/reset', '/timeout'])('bounds and classifies a transport failure: %s', async (path) => {
  await expect(api.get(origin + path, { timeout: 50 })).rejects.toMatchObject({
    name: 'ApiError',
    code: 'NETWORK_ERROR',
  });
});

it('sends the original URL as snake_case JSON and keeps cookie support enabled', async () => {
  const body = { repository_url: ' HTTPS://GitHub.com/Owner/Repo.git/ ' };
  const response = await api.post(origin + '/echo', body);
  expect(response.data).toEqual({ method: 'POST', body });
  expect(response.config.withCredentials).toBe(true);
  expect(response.config.timeout).toBe(30000);
});

it('rejects a cancelled request without awaiting the transport timeout', async () => {
  const controller = new AbortController();
  const request = api.get(origin + '/timeout', { signal: controller.signal });
  const assertion = expect(request).rejects.toBeInstanceOf(ApiError);
  controller.abort();
  await assertion;
});
