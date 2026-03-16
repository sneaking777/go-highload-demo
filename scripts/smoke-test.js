import http from 'k6/http';
import { check } from 'k6';

// Быстрый smoke-тест: 5 VU, 10 секунд.
// Запуск: docker compose run k6 run /scripts/smoke-test.js
export const options = {
  vus: 5,
  duration: '10s',
  thresholds: {
    http_req_duration: ['p(95)<300'],
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://app:8080';
const channels = ['email', 'push', 'sms', 'webhook'];

export default function () {
  // Health check
  const health = http.get(`${BASE_URL}/health`);
  check(health, { 'health: status 200': (r) => r.status === 200 });

  // Создаём уведомление
  const channel = channels[Math.floor(Math.random() * channels.length)];
  const res = http.post(
    `${BASE_URL}/notifications`,
    JSON.stringify({
      user_id: `smoke:${__VU}`,
      channel: channel,
      payload: `smoke-${Date.now()}`,
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );

  check(res, {
    'create: status 201': (r) => r.status === 201,
  });
}
