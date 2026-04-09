// POST /api/proxy — proxy requests from panel to a specific node
// Body: { nodeId, method, path, body? }
import { getDb } from '../../../lib/db';
import { nodeApi } from '../../../lib/nodeApi';

export async function POST(request) {
  const { nodeId, method, path, body } = await request.json();

  const db = getDb();
  const node = db.prepare('SELECT * FROM nodes WHERE id = ?').get(nodeId);

  if (!node) {
    return Response.json({ error: 'Node not found' }, { status: 404 });
  }

  try {
    const options = { method: method || 'GET' };
    if (body) {
      options.body = JSON.stringify(body);
    }

    const result = await nodeApi(node.address, node.admin_token, path, options);
    return Response.json(result);
  } catch (e) {
    return Response.json({ error: e.message }, { status: 502 });
  }
}
