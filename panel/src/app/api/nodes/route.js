// GET /api/nodes — list all nodes with their live status
// POST /api/nodes — add a new node
// DELETE /api/nodes?id=X — remove a node
import { getDb } from '../../../lib/db';
import { requireAdmin } from '../../../lib/auth';
import { nodeApi } from '../../../lib/nodeApi';

export async function GET() {
  const db = getDb();
  const nodes = db.prepare('SELECT * FROM nodes ORDER BY id').all();

  // Fetch live status from each node
  const results = await Promise.all(
    nodes.map(async (node) => {
      try {
        const status = await nodeApi(node.address, node.admin_token, '/api/v1/status');
        
        // Update last_detected_ip if reported
        if (status && status.server_ip && status.server_ip !== node.last_detected_ip) {
          try {
            db.prepare('UPDATE nodes SET last_detected_ip = ? WHERE id = ?').run(status.server_ip, node.id);
            node.last_detected_ip = status.server_ip;
          } catch (updateErr) {
            console.error('Failed to update last_detected_ip', updateErr);
          }
        }
        
        return { ...node, admin_token: '***', online: true, status };
      } catch (e) {
        return { ...node, admin_token: '***', online: false, error: e.message };
      }
    })
  );

  return Response.json(results);
}

export async function POST(request) {
  const admin = requireAdmin(request);
  if (!admin) return Response.json({ error: 'Admin access required' }, { status: 403 });

  const { name, address, admin_token, use_reported_ip } = await request.json();

  if (!name || !address || !admin_token) {
    return Response.json({ error: 'name, address, admin_token required' }, { status: 400 });
  }

  const useReportedIpVal = use_reported_ip === false ? 0 : 1;

  // Verify connection
  try {
    const status = await nodeApi(address, admin_token, '/api/v1/status');
    const db = getDb();
    const detectedIp = status.server_ip || null;
    const result = db.prepare('INSERT INTO nodes (name, address, admin_token, use_reported_ip, last_detected_ip) VALUES (?, ?, ?, ?, ?)').run(name, address, admin_token, useReportedIpVal, detectedIp);

    return Response.json({ id: result.lastInsertRowid, name, address }, { status: 201 });
  } catch (e) {
    return Response.json({ error: `Cannot connect to node: ${e.message}` }, { status: 400 });
  }
}

export async function DELETE(request) {
  const admin = requireAdmin(request);
  if (!admin) return Response.json({ error: 'Admin access required' }, { status: 403 });

  const { searchParams } = new URL(request.url);
  const id = searchParams.get('id');

  if (!id) {
    return Response.json({ error: 'id required' }, { status: 400 });
  }

  const db = getDb();
  db.prepare('DELETE FROM nodes WHERE id = ?').run(id);

  return Response.json({ success: true });
}

export async function PUT(request) {
  const admin = requireAdmin(request);
  if (!admin) return Response.json({ error: 'Admin access required' }, { status: 403 });

  const { id, name, address, admin_token, use_reported_ip } = await request.json();

  if (!id || !name || !address || !admin_token) {
    return Response.json({ error: 'id, name, address, admin_token required' }, { status: 400 });
  }

  const useReportedIpVal = use_reported_ip === false ? 0 : 1;

  // Verify connection (try new config)
  try {
    const status = await nodeApi(address, admin_token, '/api/v1/status');
    const db = getDb();
    const detectedIp = status.server_ip || null;
    
    db.prepare('UPDATE nodes SET name = ?, address = ?, admin_token = ?, use_reported_ip = ?, last_detected_ip = ? WHERE id = ?').run(name, address, admin_token, useReportedIpVal, detectedIp, id);

    return Response.json({ id, name, address }, { status: 200 });
  } catch (e) {
    return Response.json({ error: `Cannot connect to node with updated config: ${e.message}` }, { status: 400 });
  }
