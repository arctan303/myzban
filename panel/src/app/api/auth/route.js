// POST /api/auth — login
// GET /api/auth — check session
const { getDb } = require('../../../lib/db');

export async function POST(request) {
  const { password } = await request.json();
  const db = getDb();
  const row = db.prepare('SELECT value FROM settings WHERE key = ?').get('admin_password');

  if (!row || row.value !== password) {
    return Response.json({ error: 'Invalid password' }, { status: 401 });
  }

  // Simple token-based auth
  const token = Buffer.from(`pnm:${Date.now()}:${password}`).toString('base64');
  return Response.json({ token });
}

export async function GET(request) {
  return Response.json({ ok: true });
}
