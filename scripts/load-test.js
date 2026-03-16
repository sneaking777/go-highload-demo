import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Кастомные метрики
const createSuccess = new Rate('create_success');
const getSuccess = new Rate('get_success');
const createDuration = new Trend('create_duration', true);
const getDuration = new Trend('get_duration', true);

// Сценарии нагрузки:
// 1. smoke    — быстрая проверка (10 VU, 30s)
// 2. load     — типичная нагрузка (50 VU, 2m)
// 3. stress   — стресс-тест (200 VU, рампа до пика)
// 4. spike    — пиковая нагрузка (резкий скачок до 500 VU)
export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      tags: { scenario: 'smoke' },
      env: { SCENARIO: 'smoke' },
    },
    load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 50 },
        { duration: '1m', target: 50 },
        { duration: '30s', target: 0 },
      ],
      startTime: '35s',
      tags: { scenario: 'load' },
      env: { SCENARIO: 'load' },
    },
    stress: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 100 },
        { duration: '1m', target: 200 },
        { duration: '30s', target: 0 },
      ],
      startTime: '2m30s',
      tags: { scenario: 'stress' },
      env: { SCENARIO: 'stress' },
    },
    spike: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 500 },
        { duration: '30s', target: 500 },
        { duration: '10s', target: 0 },
      ],
      startTime: '5m',
      tags: { scenario: 'spike' },
      env: { SCENARIO: 'spike' },
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    create_success: ['rate>0.95'],
    get_success: ['rate>0.95'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://app:8080';
const channels = ['email', 'push', 'sms', 'webhook'];

function randomChannel() {
  return channels[Math.floor(Math.random() * channels.length)];
}

export default function () {
  // POST /notifications
  const payload = JSON.stringify({
    user_id: `user:${__VU}`,
    channel: randomChannel(),
    payload: `load-test-message-${Date.now()}`,
  });

  const createRes = http.post(`${BASE_URL}/notifications`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const created = check(createRes, {
    'create: status 201': (r) => r.status === 201,
    'create: has id': (r) => {
      try { return JSON.parse(r.body).id !== ''; } catch { return false; }
    },
  });
  createSuccess.add(created);
  createDuration.add(createRes.timings.duration);

  if (created) {
    const id = JSON.parse(createRes.body).id;

    // Пауза для async-обработки воркером
    sleep(0.1);

    // GET /notifications/{id}
    const getRes = http.get(`${BASE_URL}/notifications/${id}`);
    const got = check(getRes, {
      'get: status 200': (r) => r.status === 200,
      'get: correct id': (r) => {
        try { return JSON.parse(r.body).id === id; } catch { return false; }
      },
    });
    getSuccess.add(got);
    getDuration.add(getRes.timings.duration);
  }

  sleep(0.05);
}
