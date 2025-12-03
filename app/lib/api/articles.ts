import { Hono } from 'hono';
import type { ApiEnv } from './index';
import type { Article, CreateArticleInput, UpdateArticleInput } from '../db/schema';

export const articlesRouter = new Hono<ApiEnv>();

// Generate UUID
const generateId = () => crypto.randomUUID();

// Generate slug from title
const generateSlug = (title: string): string => {
  return title
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .trim();
};

// List articles
articlesRouter.get('/', async (c) => {
  const db = c.env.hss_science_db;
  const status = c.req.query('status'); // ?status=published
  const limit = parseInt(c.req.query('limit') || '20');
  const offset = parseInt(c.req.query('offset') || '0');

  let query = `
    SELECT 
      a.*,
      u.id as author_id,
      u.name as author_name,
      u.avatar_url as author_avatar
    FROM articles a
    LEFT JOIN users u ON a.author_id = u.id
  `;
  const params: string[] = [];

  if (status) {
    query += ' WHERE a.status = ?';
    params.push(status);
  }

  query += ' ORDER BY a.created_at DESC LIMIT ? OFFSET ?';
  params.push(limit.toString(), offset.toString());

  const result = await db.prepare(query).bind(...params).all();

  const articles = result.results.map((row: any) => ({
    ...row,
    author: row.author_id ? {
      id: row.author_id,
      name: row.author_name,
      avatar_url: row.author_avatar,
    } : null,
  }));

  return c.json({ articles, total: result.results.length });
});

// Get single article by ID or slug
articlesRouter.get('/:idOrSlug', async (c) => {
  const db = c.env.hss_science_db;
  const idOrSlug = c.req.param('idOrSlug');

  const result = await db.prepare(`
    SELECT 
      a.*,
      u.id as author_id,
      u.name as author_name,
      u.avatar_url as author_avatar
    FROM articles a
    LEFT JOIN users u ON a.author_id = u.id
    WHERE a.id = ? OR a.slug = ?
  `).bind(idOrSlug, idOrSlug).first();

  if (!result) {
    return c.json({ error: 'Article not found' }, 404);
  }

  const article = {
    ...result,
    author: result.author_id ? {
      id: result.author_id,
      name: result.author_name,
      avatar_url: result.author_avatar,
    } : null,
  };

  return c.json({ article });
});

// Create article
articlesRouter.post('/', async (c) => {
  const db = c.env.hss_science_db;
  const body = await c.req.json<CreateArticleInput>();

  if (!body.title) {
    return c.json({ error: 'Title is required' }, 400);
  }

  const id = generateId();
  const slug = body.slug || generateSlug(body.title);
  const now = new Date().toISOString();

  await db.prepare(`
    INSERT INTO articles (id, slug, title, content, excerpt, cover_image_url, status, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
  `).bind(
    id,
    slug,
    body.title,
    body.content || null,
    body.excerpt || null,
    body.cover_image_url || null,
    body.status || 'draft',
    now,
    now
  ).run();

  const article = await db.prepare('SELECT * FROM articles WHERE id = ?').bind(id).first();

  return c.json({ article }, 201);
});

// Update article
articlesRouter.put('/:id', async (c) => {
  const db = c.env.hss_science_db;
  const id = c.req.param('id');
  const body = await c.req.json<UpdateArticleInput>();

  // Check if article exists
  const existing = await db.prepare('SELECT * FROM articles WHERE id = ?').bind(id).first();
  if (!existing) {
    return c.json({ error: 'Article not found' }, 404);
  }

  const updates: string[] = [];
  const values: (string | null)[] = [];

  if (body.title !== undefined) {
    updates.push('title = ?');
    values.push(body.title);
  }
  if (body.slug !== undefined) {
    updates.push('slug = ?');
    values.push(body.slug);
  }
  if (body.content !== undefined) {
    updates.push('content = ?');
    values.push(body.content);
  }
  if (body.excerpt !== undefined) {
    updates.push('excerpt = ?');
    values.push(body.excerpt);
  }
  if (body.cover_image_url !== undefined) {
    updates.push('cover_image_url = ?');
    values.push(body.cover_image_url);
  }
  if (body.status !== undefined) {
    updates.push('status = ?');
    values.push(body.status);
    // Set published_at when publishing
    if (body.status === 'published' && !existing.published_at) {
      updates.push('published_at = ?');
      values.push(new Date().toISOString());
    }
  }
  if (body.published_at !== undefined) {
    updates.push('published_at = ?');
    values.push(body.published_at);
  }

  if (updates.length === 0) {
    return c.json({ error: 'No fields to update' }, 400);
  }

  updates.push('updated_at = ?');
  values.push(new Date().toISOString());
  values.push(id);

  await db.prepare(`
    UPDATE articles SET ${updates.join(', ')} WHERE id = ?
  `).bind(...values).run();

  const article = await db.prepare('SELECT * FROM articles WHERE id = ?').bind(id).first();

  return c.json({ article });
});

// Delete article
articlesRouter.delete('/:id', async (c) => {
  const db = c.env.hss_science_db;
  const id = c.req.param('id');

  const existing = await db.prepare('SELECT * FROM articles WHERE id = ?').bind(id).first();
  if (!existing) {
    return c.json({ error: 'Article not found' }, 404);
  }

  await db.prepare('DELETE FROM articles WHERE id = ?').bind(id).run();

  return c.json({ success: true });
});
