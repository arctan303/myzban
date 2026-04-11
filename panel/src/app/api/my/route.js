// GET /api/my — get current user's proxy info across all nodes
import { getDb } from '../../../lib/db';
import { requireAuth } from '../../../lib/auth';
import { nodeApi } from '../../../lib/nodeApi';

export async function GET(request) {
  const authUser = requireAuth(request);
  if (!authUser) {
    return Response.json({ error: 'Unauthorized' }, { status: 401 });
  }

  const proxyUsername = authUser.proxy_username;
  if (!proxyUsername) {
    return Response.json({ error: '该账号未关联代理用户' }, { status: 404 });
  }

  const db = getDb();
  const nodes = db.prepare('SELECT * FROM nodes ORDER BY id').all();

  // Fetch this user's info from all nodes in parallel
  const results = await Promise.all(
    nodes.map(async (node) => {
      try {
        const status = await nodeApi(node.address, node.admin_token, '/api/v1/status');
        const users = await nodeApi(node.address, node.admin_token, '/api/v1/users');
        const user = Array.isArray(users) ? users.find(u => u.username === proxyUsername) : null;

        if (!user) return null;

        return {
          node_id: node.id,
          node_name: node.name,
          online: true,
          vless_running: status.vless?.installed && status.vless?.running,
          hy2_running: status.hysteria2?.installed && status.hysteria2?.running,
          traffic_up: user.traffic_up,
          traffic_down: user.traffic_down,
          sub_token: user.sub_token,
          uuid: user.uuid,
          hy2_password: user.hy2_password,
          enabled: user.enabled,
        };
      } catch {
        return null;
      }
    })
  );

  const nodeData = results.filter(Boolean);

  // Total traffic across all nodes
  const totalUp = nodeData.reduce((sum, n) => sum + (n.traffic_up || 0), 0);
  const totalDown = nodeData.reduce((sum, n) => sum + (n.traffic_down || 0), 0);

  return Response.json({
    username: proxyUsername,
    total_traffic_up: totalUp,
    total_traffic_down: totalDown,
    nodes: nodeData,
  });
}
