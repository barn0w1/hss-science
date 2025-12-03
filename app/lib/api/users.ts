import { Hono } from 'hono';
import type { ApiEnv } from './index';
import type { CreateUserInput, User } from '../db/schema';

export const usersRouter = new Hono<ApiEnv>();

const generateId = () => crypto.randomUUID();

// List users
usersRouter.get('/', async (c) => {
  const db = c.env.hss_science_db;

  const result = await db.prepare(`
    SELECT id, email, name, avatar_url, role, created_at, updated_at
    FROM users
    ORDER BY created_at DESC
  `).all();

  return c.json({ users: result.results });
});

// Get single user
usersRouter.get('/:id', async (c) => {
  const db = c.env.hss_science_db;
  const id = c.req.param('id');

  const user = await db.prepare(`
    SELECT id, email, name, avatar_url, role, created_at, updated_at
    FROM users WHERE id = ?
  `).bind(id).first();

  if (!user) {
    return c.json({ error: 'User not found' }, 404);
  }

  return c.json({ user });
});

// Get user by email (for Cloudflare Access integration)
usersRouter.get('/email/:email', async (c) => {
  const db = c.env.hss_science_db;
  const email = c.req.param('email');

  const user = await db.prepare(`
    SELECT id, email, name, avatar_url, role, created_at, updated_at
    FROM users WHERE email = ?
  `).bind(email).first();

  if (!user) {
    return c.json({ error: 'User not found' }, 404);
  }

  return c.json({ user });
});

// Create or update user (upsert - for Cloudflare Access login)
usersRouter.post('/sync', async (c) => {
  const db = c.env.hss_science_db;
  const body = await c.req.json<CreateUserInput>();

  if (!body.email || !body.name) {
    return c.json({ error: 'Email and name are required' }, 400);
  }

  // Check if user exists
  const existing = await db.prepare('SELECT * FROM users WHERE email = ?').bind(body.email).first();

  const now = new Date().toISOString();

  if (existing) {
    // Update existing user
    await db.prepare(`
      UPDATE users SET name = ?, avatar_url = ?, updated_at = ? WHERE email = ?
    `).bind(body.name, body.avatar_url || null, now, body.email).run();
  } else {
    // Create new user
    const id = generateId();
    await db.prepare(`
      INSERT INTO users (id, email, name, avatar_url, role, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?)
    `).bind(
      id,
      body.email,
      body.name,
      body.avatar_url || null,
      body.role || 'editor',
      now,
      now
    ).run();
  }

  const user = await db.prepare(`
    SELECT id, email, name, avatar_url, role, created_at, updated_at
    FROM users WHERE email = ?
  `).bind(body.email).first();

  return c.json({ user }, existing ? 200 : 201);
});

// Update user role (admin only)
usersRouter.put('/:id/role', async (c) => {
  const db = c.env.hss_science_db;
  const id = c.req.param('id');
  const body = await c.req.json<{ role: User['role'] }>();

  if (!['admin', 'editor', 'viewer'].includes(body.role)) {
    return c.json({ error: 'Invalid role' }, 400);
  }

  const existing = await db.prepare('SELECT * FROM users WHERE id = ?').bind(id).first();
  if (!existing) {
    return c.json({ error: 'User not found' }, 404);
  }

  await db.prepare(`
    UPDATE users SET role = ?, updated_at = ? WHERE id = ?
  `).bind(body.role, new Date().toISOString(), id).run();

  const user = await db.prepare(`
    SELECT id, email, name, avatar_url, role, created_at, updated_at
    FROM users WHERE id = ?
  `).bind(id).first();

  return c.json({ user });
});

// Delete user
usersRouter.delete('/:id', async (c) => {
  const db = c.env.hss_science_db;
  const id = c.req.param('id');

  const existing = await db.prepare('SELECT * FROM users WHERE id = ?').bind(id).first();
  if (!existing) {
    return c.json({ error: 'User not found' }, 404);
  }

  await db.prepare('DELETE FROM users WHERE id = ?').bind(id).run();

  return c.json({ success: true });
});
