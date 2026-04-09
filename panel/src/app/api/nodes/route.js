// GET /api/nodes — list all nodes with their live status
// POST /api/nodes — add a new node
// DELETE /api/nodes?id=X — remove a node
import { getDb } from '../../../lib/db';
import { nodeApi } from '../../../lib/nodeApi';

export async function GET() {
  const db = getDb();
  const nodes = db.prepare('SELECT * FROM nodes ORDER BY id').all();

  // Fetch live status from each node
  const results = await Promise.all(
    nodes.map(async (node) => {
      try {
        const status = await nodeApi(node.address, node.admin_token, '/api/v1/status');
        return { ...node, admin_token: '***', online: true, status };
      } catch (e) {
        return { ...node, admin_token: '***', online: false, error: e.message };
      }
    })
  );

  return Response.json(results);
}

export async function POST(request) {
  const { name, address, admin_token } = await request.json();

  if (!name || !address || !admin_token) {
    return Response.json({ error: 'name, address, admin_token required' }, { status: 400 });
  }

  // Verify connection
  try {
    await nodeApi(address, admin_token, '/api/v1/status');
  } catch (e) {
    return Response.json({ error: `Cannot connect to node: ${e.message}` }, { status: 400 });
  }

  const db = getDb();
  const result = db.prepare('INSERT INTO nodes (name, address, admin_token) VALUES (?, ?, ?)').run(name, address, admin_token);

  return Response.json({ id: result.lastInsertRowid, name, address }, { status: 201 });
}

export async function DELETE(request) {
  const { searchParams } = new URL(request.url);
  const id = searchParams.get('id');

  if (!id) {
    return Response.json({ error: 'id required' }, { status: 400 });
  }

  const db = getDb();
  db.prepare('DELETE FROM nodes WHERE id = ?').run(id);
  return Response.json({ status: 'deleted' });
}
