import pino from "pino";

export const logger = pino({
  level: process.env.LOG_LEVEL ?? "info",
  redact: ["cookie", "authorization", "*.cookie", "*.authorization"],
});

export function createRequestLogger(request: Request) {
  const url = new URL(request.url);
  return logger.child({
    method: request.method,
    pathname: url.pathname,
    requestId: request.headers.get("x-request-id") ?? undefined,
  });
}
