// Database schema types

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url: string | null;
  role: 'admin' | 'editor' | 'viewer';
  created_at: string;
  updated_at: string;
}

export interface Article {
  id: string;
  slug: string;
  title: string;
  content: string | null;  // Tiptap JSON
  excerpt: string | null;
  cover_image_url: string | null;
  status: 'draft' | 'published';
  author_id: string | null;
  published_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface Media {
  id: string;
  filename: string;
  original_filename: string | null;
  mime_type: string | null;
  size: number | null;
  r2_key: string;
  url: string;
  alt_text: string | null;
  uploaded_by: string | null;
  created_at: string;
}

// API response types
export interface ArticleWithAuthor extends Article {
  author?: Pick<User, 'id' | 'name' | 'avatar_url'> | null;
}

// Input types for creating/updating
export interface CreateArticleInput {
  title: string;
  slug?: string;
  content?: string;
  excerpt?: string;
  cover_image_url?: string;
  status?: 'draft' | 'published';
}

export interface UpdateArticleInput {
  title?: string;
  slug?: string;
  content?: string;
  excerpt?: string;
  cover_image_url?: string;
  status?: 'draft' | 'published';
  published_at?: string;
}

export interface CreateUserInput {
  email: string;
  name: string;
  avatar_url?: string;
  role?: 'admin' | 'editor' | 'viewer';
}
