import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '5s', target: 50 },   // Warm-up to 50 users
    { duration: '15s', target: 200 }, // Spike to 200 concurrent connections
    { duration: '5s', target: 0 },    // Ramp-down
  ],
  thresholds: {
    http_req_duration: ['p(95)<10', 'p(99)<20'], // 99% requests under 20ms
  },
};

export default function () {
  const params = {
    headers: {
      'X-User-Tier': 'enterprise',
    },
  };

  const res = http.get('http://localhost:8080/api/v1/ping', params);

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'has rate limit header': (r) => r.headers['X-Ratelimit-Limit'] !== undefined,
  });

  sleep(0.01);
}