import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { articlesRouter } from './articles';
import { mediaRouter } from './media';
import { usersRouter } from './users';

// API app type with Cloudflare bindings
export type ApiEnv = {
  Bindings: Env;
};

// Create API router with /api base path
export const api = new Hono<ApiEnv>().basePath('/api');

// Middleware
api.use('*', cors());

// Health check
api.get('/health', (c) => {
  return c.json({ status: 'ok', timestamp: new Date().toISOString() });
});

// Mount routers
api.route('/articles', articlesRouter);
api.route('/media', mediaRouter);
api.route('/users', usersRouter);

// 404 handler
api.notFound((c) => {
  return c.json({ error: 'Not found' }, 404);
});

// Error handler
api.onError((err, c) => {
  console.error('API Error:', err);
  return c.json({ error: 'Internal server error' }, 500);
});
