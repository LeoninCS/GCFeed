import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";

const BASE_URL = (__ENV.BASE_URL || "http://127.0.0.1:8080").replace(/\/$/, "");
const SCENE = __ENV.SCENE || "recommend";
const TOKEN = __ENV.TOKEN || "";
const ACCOUNT = __ENV.ACCOUNT || "";
const PASSWORD = __ENV.PASSWORD || "";
const LIMIT = Number(__ENV.LIMIT || 10);
const THINK_TIME = Number(__ENV.THINK_TIME || 1);

export const options = {
  vus: Number(__ENV.VUS || 20),
  duration: __ENV.DURATION || "60s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"],
    feed_success_rate: ["rate>0.99"]
  }
};

const feedItems = new Counter("feed_items");
const feedSuccessRate = new Rate("feed_success_rate");
const feedDuration = new Trend("feed_duration");

export function setup() {
  if (TOKEN) {
    return { token: TOKEN };
  }
  if (!ACCOUNT && !PASSWORD) {
    return { token: "" };
  }
  if (!ACCOUNT || !PASSWORD) {
    throw new Error("ACCOUNT and PASSWORD must be provided together");
  }

  const response = http.post(
    `${BASE_URL}/api/sessions`,
    JSON.stringify({
      account: ACCOUNT,
      password: PASSWORD
    }),
    {
      headers: {
        "Content-Type": "application/json"
      },
      tags: {
        endpoint: "login"
      }
    }
  );

  if (response.status !== 200) {
    throw new Error(`login failed: status=${response.status} body=${response.body}`);
  }

  const body = parseJSON(response);
  if (!body.access_token) {
    throw new Error("login failed: missing access_token");
  }
  return { token: body.access_token };
}

export default function (data) {
  const response = requestFeed(data?.token || "");
  const ok = check(response, {
    "status is 200": (res) => res.status === 200,
    "has feed items array": (res) => Array.isArray(parseJSON(res).items)
  });

  feedSuccessRate.add(ok);
  feedDuration.add(response.timings.duration);

  const body = parseJSON(response);
  if (Array.isArray(body.items)) {
    feedItems.add(body.items.length);
  }

  sleep(THINK_TIME);
}

function requestFeed(token) {
  if (SCENE === "recommend") {
    return http.post(
      `${BASE_URL}/api/feed-queries`,
      JSON.stringify({
        scene: SCENE,
        limit: LIMIT,
        context: {
          request_id: `k6-${SCENE}-${Date.now()}-${__VU}-${__ITER}`
        }
      }),
      requestParams(token)
    );
  }

  const url = `${BASE_URL}/api/feed-items?scene=${encodeURIComponent(SCENE)}&limit=${encodeURIComponent(String(LIMIT))}`;
  return http.get(url, requestParams(token));
}

function requestParams(token) {
  const headers = {
    "Content-Type": "application/json"
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return {
    headers,
    tags: {
      scene: SCENE
    }
  };
}

function parseJSON(response) {
  try {
    return response.json();
  } catch {
    return {};
  }
}
