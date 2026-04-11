import { getDb } from '../../../lib/db';
import { requireAdmin } from '../../../lib/auth';

export async function GET(request) {
  const admin = requireAdmin(request);
  if (!admin) {
    return Response.json({ error: 'Admin access required' }, { status: 403 });
  }

  const db = getDb();
  let row = db.prepare("SELECT value FROM settings WHERE key = 'system_yaml_template'").get();
  
  return Response.json({ template: row ? row.value : '' });
}

export async function POST(request) {
  const admin = requireAdmin(request);
  if (!admin) {
    return Response.json({ error: 'Admin access required' }, { status: 403 });
  }

  const { template } = await request.json();
  if (typeof template !== 'string') {
    return Response.json({ error: 'Invalid template' }, { status: 400 });
  }

  const db = getDb();
  db.prepare("INSERT INTO settings (key, value) VALUES ('system_yaml_template', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value")
    .run(template);

  return Response.json({ status: 'ok' });
}
