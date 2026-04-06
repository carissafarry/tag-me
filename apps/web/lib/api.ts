const DEFAULT_API_BASE_URL = "http://localhost:8080";

export type ScannerMessageType =
  | "blocking"
  | "illegal_parking"
  | "lights_on"
  | "custom";

export type ConversationLifecycleStatus =
  | "PENDING"
  | "DELIVERED"
  | "OPENED"
  | "ON_THE_WAY"
  | "RESOLVED";

export interface SendMessagePayload {
  qr_token: string;
  message_type: ScannerMessageType;
  content?: string;
  location_latitude?: number;
  location_longitude?: number;
  location_text?: string;
}

export interface CreateMessageResponse {
  conversation_id: string;
  message_id: string;
  status: ConversationLifecycleStatus;
  created_at: string;
}

export interface ConversationStatusResponse {
  conversation_id: string;
  status: ConversationLifecycleStatus;
  created_at: string;
}

export interface ApiErrorPayload {
  error?: string;
  code?: string;
  detail?: string;
}

interface ApiRequestOptions extends Omit<RequestInit, "body" | "method"> {
  body?: BodyInit | null;
  headers?: HeadersInit;
  method?: "GET" | "POST" | "PATCH" | "PUT" | "DELETE";
}

export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly detail?: string;
  readonly payload?: ApiErrorPayload;

  constructor(
    message: string,
    status: number,
    options?: {
      code?: string;
      detail?: string;
      payload?: ApiErrorPayload;
    },
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = options?.code;
    this.detail = options?.detail;
    this.payload = options?.payload;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (isApiError(error)) {
    return error.message || fallback;
  }

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return fallback;
}

function resolveApiBaseUrl() {
  return (
    process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, "") ??
    DEFAULT_API_BASE_URL
  );
}

function buildApiUrl(path: string) {
  if (!path.startsWith("/")) {
    throw new Error(`API path must start with "/": ${path}`);
  }

  return `${resolveApiBaseUrl()}${path}`;
}

function createHeaders(headers?: HeadersInit) {
  const mergedHeaders = new Headers(headers);

  if (!mergedHeaders.has("Accept")) {
    mergedHeaders.set("Accept", "application/json");
  }

  return mergedHeaders;
}

async function parseResponseBody(response: Response) {
  const contentType = response.headers.get("content-type") ?? "";

  if (contentType.includes("application/json")) {
    return response.json().catch(() => null);
  }

  const text = await response.text().catch(() => "");
  return text ? { error: text } : null;
}

class ApiClient {
  constructor(private readonly baseUrl: string) {}

  async request<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
    const headers = createHeaders(options.headers);

    const response = await fetch(`${this.baseUrl}${path}`, {
      cache: "no-store",
      ...options,
      headers,
    });

    const body = await parseResponseBody(response);

    if (!response.ok) {
      const payload =
        body && typeof body === "object" ? (body as ApiErrorPayload) : undefined;

      throw new ApiError(
        payload?.error || `Request failed with status ${response.status}`,
        response.status,
        {
          code: payload?.code,
          detail: payload?.detail,
          payload,
        },
      );
    }

    return body as T;
  }

  get<T>(path: string, options?: Omit<ApiRequestOptions, "method" | "body">) {
    return this.request<T>(path, {
      ...options,
      method: "GET",
    });
  }

  post<T>(
    path: string,
    body?: unknown,
    options?: Omit<ApiRequestOptions, "method" | "body">,
  ) {
    const headers = createHeaders(options?.headers);

    if (body !== undefined && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }

    return this.request<T>(path, {
      ...options,
      method: "POST",
      headers,
      body: body === undefined ? null : JSON.stringify(body),
    });
  }
}

export const apiClient = new ApiClient(resolveApiBaseUrl());

export function getApiUrl(path: string) {
  return buildApiUrl(path);
}

export function sendMessage(payload: SendMessagePayload) {
  return apiClient.post<CreateMessageResponse>("/messages", payload);
}

export function getConversationStatus(conversationId: string) {
  return apiClient.get<ConversationStatusResponse>(
    `/conversations/${encodeURIComponent(conversationId)}/status`,
  );
}
