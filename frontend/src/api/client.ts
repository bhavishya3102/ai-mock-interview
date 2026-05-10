import { env } from "@/lib/env";
import { getAuthToken } from "@/lib/tokenStore";
import type { ApiErrorBody } from "@/types/api";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, message: string, code: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export class UnauthorizedError extends ApiError {
  constructor(message = "Unauthorized") {
    super(401, message, "UNAUTHORIZED");
    this.name = "UnauthorizedError";
  }
}

export interface ApiFetchInit extends Omit<RequestInit, "body"> {
  body?: unknown;
  signal?: AbortSignal;
}

function isFormData(value: unknown): value is FormData {
  return typeof FormData !== "undefined" && value instanceof FormData;
}

function isApiErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as { error?: unknown };
  if (typeof candidate.error !== "object" || candidate.error === null) return false;
  const err = candidate.error as { code?: unknown; message?: unknown };
  return typeof err.code === "string" && typeof err.message === "string";
}

export async function apiFetch<T>(path: string, init: ApiFetchInit = {}): Promise<T> {
  const token = await getAuthToken();

  const headers = new Headers(init.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);

  let body: BodyInit | undefined;
  if (init.body !== undefined && init.body !== null) {
    if (isFormData(init.body) || init.body instanceof Blob || typeof init.body === "string") {
      body = init.body;
    } else {
      body = JSON.stringify(init.body);
      if (!headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
      }
    }
  }

  const fetchInit: RequestInit = {
    headers,
    ...(init.method ? { method: init.method } : {}),
    ...(init.signal ? { signal: init.signal } : {}),
    ...(init.credentials ? { credentials: init.credentials } : {}),
    ...(init.cache ? { cache: init.cache } : {}),
    ...(init.mode ? { mode: init.mode } : {}),
    ...(init.redirect ? { redirect: init.redirect } : {}),
    ...(init.referrer ? { referrer: init.referrer } : {}),
    ...(init.referrerPolicy ? { referrerPolicy: init.referrerPolicy } : {}),
    ...(init.integrity ? { integrity: init.integrity } : {}),
    ...(init.keepalive !== undefined ? { keepalive: init.keepalive } : {}),
    ...(body !== undefined ? { body } : {}),
  };

  const url = `${env.VITE_API_BASE_URL}${path}`;
  const response = await fetch(url, fetchInit);

  if (response.status === 204) {
    return null as T;
  }

  const contentType = response.headers.get("content-type") ?? "";
  const isJson = contentType.includes("application/json");
  const payload: unknown = isJson ? await response.json() : await response.text();

  if (!response.ok) {
    if (response.status === 401) {
      const message = isApiErrorBody(payload) ? payload.error.message : "Unauthorized";
      throw new UnauthorizedError(message);
    }
    if (isApiErrorBody(payload)) {
      throw new ApiError(response.status, payload.error.message, payload.error.code);
    }
    throw new ApiError(response.status, response.statusText || "Request failed", "UNKNOWN");
  }

  return payload as T;
}
