import { Hono } from 'hono';
import type { ApiEnv } from './index';
import type { Media } from '../db/schema';

export const mediaRouter = new Hono<ApiEnv>();

const generateId = () => crypto.randomUUID();

// List media
mediaRouter.get('/', async (c) => {
  const db = c.env.hss_science_db;
  const limit = parseInt(c.req.query('limit') || '50');
  const offset = parseInt(c.req.query('offset') || '0');
  const mimeType = c.req.query('type'); // ?type=image

  let query = 'SELECT * FROM media';
  const params: string[] = [];

  if (mimeType) {
    query += ' WHERE mime_type LIKE ?';
    params.push(`${mimeType}%`);
  }

  query += ' ORDER BY created_at DESC LIMIT ? OFFSET ?';
  params.push(limit.toString(), offset.toString());

  const result = await db.prepare(query).bind(...params).all();

  return c.json({ media: result.results });
});

// Get single media
mediaRouter.get('/:id', async (c) => {
  const db = c.env.hss_science_db;
  const id = c.req.param('id');

  const media = await db.prepare('SELECT * FROM media WHERE id = ?').bind(id).first();

  if (!media) {
    return c.json({ error: 'Media not found' }, 404);
  }

  return c.json({ media });
});

// Upload media
mediaRouter.post('/upload', async (c) => {
  const db = c.env.hss_science_db;
  const r2 = c.env.hss_science_media;

  const formData = await c.req.formData();
  const file = formData.get('file') as File | null;
  const altText = formData.get('alt_text') as string | null;

  if (!file) {
    return c.json({ error: 'No file provided' }, 400);
  }

  // Validate file type
  const allowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/svg+xml'];
  if (!allowedTypes.includes(file.type)) {
    return c.json({ error: 'Invalid file type. Allowed: jpeg, png, gif, webp, svg' }, 400);
  }

  // Generate unique filename
  const id = generateId();
  const ext = file.name.split('.').pop() || 'bin';
  const filename = `${id}.${ext}`;
  const r2Key = `uploads/${new Date().getFullYear()}/${filename}`;

  // Upload to R2
  const arrayBuffer = await file.arrayBuffer();
  await r2.put(r2Key, arrayBuffer, {
    httpMetadata: {
      contentType: file.type,
    },
  });

  // TODO: Replace with actual public URL when custom domain is configured
  const url = `/api/media/file/${id}`;

  // Save metadata to D1
  const now = new Date().toISOString();
  await db.prepare(`
    INSERT INTO media (id, filename, original_filename, mime_type, size, r2_key, url, alt_text, created_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
  `).bind(
    id,
    filename,
    file.name,
    file.type,
    file.size,
    r2Key,
    url,
    altText,
    now
  ).run();

  const media = await db.prepare('SELECT * FROM media WHERE id = ?').bind(id).first();

  return c.json({ media }, 201);
});

// Serve media file from R2
mediaRouter.get('/file/:id', async (c) => {
  const db = c.env.hss_science_db;
  const r2 = c.env.hss_science_media;
  const id = c.req.param('id');

  const media = await db.prepare('SELECT * FROM media WHERE id = ?').bind(id).first<Media>();

  if (!media) {
    return c.json({ error: 'Media not found' }, 404);
  }

  const object = await r2.get(media.r2_key);

  if (!object) {
    return c.json({ error: 'File not found in storage' }, 404);
  }

  const headers = new Headers();
  headers.set('Content-Type', media.mime_type || 'application/octet-stream');
  headers.set('Cache-Control', 'public, max-age=31536000'); // 1 year cache

  return new Response(object.body, { headers });
});

// Delete media
mediaRouter.delete('/:id', async (c) => {
  const db = c.env.hss_science_db;
  const r2 = c.env.hss_science_media;
  const id = c.req.param('id');

  const media = await db.prepare('SELECT * FROM media WHERE id = ?').bind(id).first<Media>();

  if (!media) {
    return c.json({ error: 'Media not found' }, 404);
  }

  // Delete from R2
  await r2.delete(media.r2_key);

  // Delete from D1
  await db.prepare('DELETE FROM media WHERE id = ?').bind(id).run();

  return c.json({ success: true });
});
