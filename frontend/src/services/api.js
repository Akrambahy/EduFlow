/**
 * Centralized API client for EduFlow.
 *
 * All HTTP calls to the Go backend go through this module.
 * Base URL is configured via VITE_API_BASE_URL environment variable.
 *
 * When the backend is unavailable, the app falls back to demo data
 * provided by src/data/*.js files. This file itself never imports
 * demo data — that switch happens in the service adapters.
 */

const BASE_URL = import.meta.env.VITE_API_BASE_URL || "/api";

class ApiError extends Error {
  constructor(message, status, data) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.data = data;
  }
}

async function request(endpoint, options = {}) {
  const url = `${BASE_URL}${endpoint}`;
  const headers = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  // Attach auth token if present
  const token = localStorage.getItem("eduflow_token");
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const config = {
    ...options,
    headers,
  };

  if (options.body && typeof options.body === "object") {
    config.body = JSON.stringify(options.body);
  }

  const response = await fetch(url, config);

  if (!response.ok) {
    const data = await response.json().catch(() => null);
    throw new ApiError(
      data?.message || `Request failed with status ${response.status}`,
      response.status,
      data,
    );
  }

  // 204 No Content
  if (response.status === 204) return null;

  return response.json();
}

export const api = {
  get: (endpoint, options) => request(endpoint, { ...options, method: "GET" }),
  post: (endpoint, body, options) =>
    request(endpoint, { ...options, method: "POST", body }),
  put: (endpoint, body, options) =>
    request(endpoint, { ...options, method: "PUT", body }),
  patch: (endpoint, body, options) =>
    request(endpoint, { ...options, method: "PATCH", body }),
  delete: (endpoint, options) =>
    request(endpoint, { ...options, method: "DELETE" }),
};

export { ApiError };
export default api;
