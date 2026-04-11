import { getDb } from '../../../../../lib/db';
import { nodeApi } from '../../../../../lib/nodeApi';

export async function GET(request, { params }) {
  const { subToken } = params;

  if (!subToken) {
    return new Response('Token Not Found', { status: 404 });
  }

  const db = getDb();
  
  // 1. Authenticate user by sub_token
  const pUser = db.prepare('SELECT proxy_username FROM panel_users WHERE sub_token = ?').get(subToken);
  if (!pUser || !pUser.proxy_username) {
    return new Response('User Not Found or Has No Proxy Account', { status: 404 });
  }
  const proxyUsername = pUser.proxy_username;

  // 2. Fetch all online nodes
  const nodes = db.prepare('SELECT * FROM nodes').all();
  if (nodes.length === 0) {
    return new Response('No Nodes Available', { status: 503 });
  }

  // 3. Build dynamic configuration across all nodes
  let proxiesBlock = [];
  let proxyNamesBlock = [];
  let totalUp = 0;
  let totalDown = 0;
  let totalLimit = 0;

  const promises = nodes.map(async (node) => {
    try {
      const users = await nodeApi(node.address, node.admin_token, '/api/v1/users', {}, 3000);
      const user = users.find(u => u.username === proxyUsername);

      if (!user || (!user.enabled)) return;
      
      totalUp += user.traffic_up;
      totalDown += user.traffic_down;
      if (user.traffic_limit > totalLimit) totalLimit = user.traffic_limit; // Just use max limit across nodes, or could sum them if distributed. Using sum to be safe.
      
      if (user.traffic_limit > 0 && (user.traffic_up + user.traffic_down >= user.traffic_limit)) return;

      const status = await nodeApi(node.address, node.admin_token, '/api/v1/status', {}, 3000);
      const nodeDetails = await nodeApi(node.address, node.admin_token, '/api/v1/node', {}, 3000);

      const serverHost = new URL(node.address).hostname;

      if (status.vless?.installed && status.vless?.running) {
        const name = `${node.name}-TCP`;
        proxiesBlock.push(`  - name: "${name}"
    type: vless
    server: ${serverHost}
    port: ${status.vless.port}
    uuid: ${user.uuid}
    udp: true
    tls: true
    flow: xtls-rprx-vision
    network: tcp
    servername: ${nodeDetails.dest_domain || 'www.cloudflare.com'}
    client-fingerprint: chrome
    reality-opts:
      public-key: ${nodeDetails.xray_pub_key}
      short-id: ${nodeDetails.short_id}`);
        proxyNamesBlock.push(`      - "${name}"`);
      }

      if (status.hysteria2?.installed && status.hysteria2?.running) {
        const name = `${node.name}-UDP`;
        proxiesBlock.push(`  - name: "${name}"
    type: hysteria2
    server: ${serverHost}
    port: ${status.hysteria2.port}
    password: ${user.hy2_password}
    up: 1000 Mbps
    down: 1000 Mbps
    sni: ${nodeDetails.dest_domain || 'www.cloudflare.com'}
    skip-cert-verify: true
    alpn:
      - h3`);
        proxyNamesBlock.push(`      - "${name}"`);
      }
    } catch (e) {
      // Ignore offline nodes
      console.error(`Failed fetching from node ${node.name}:`, e.message);
    }
  });

  await Promise.all(promises);

  if (proxiesBlock.length === 0) {
    return new Response('# No active proxy protocols found globally for this user\n', { status: 200 });
  }

  // 4. Read system template
  const tmplRow = db.prepare("SELECT value FROM settings WHERE key = 'system_yaml_template'").get();
  if (!tmplRow) {
    return new Response('System template configuration err', { status: 500 });
  }

  let finalYaml = tmplRow.value;

  // 5. Inject
  finalYaml = finalYaml.replace('<__PROXIES__>', proxiesBlock.join('\n\n'));
  
  // Replace <__PROXY_NAMES__> globally (it can appear multiple times)
  const namesStr = proxyNamesBlock.join('\n');
  finalYaml = finalYaml.split('<__PROXY_NAMES__>').join(namesStr);

  // If no limits are set across any nodes, provide a large default total so the UI bar isn't confused, or omit total.
  // Standard limits: total is sum across nodes if any. But here we just use what we collected.
  // Wait, if no limit (totalLimit === 0), it's infinite, usually represented as a very large number or just omitted.
  // But let's supply large number if 0.
  const displayTotal = totalLimit > 0 ? totalLimit : 1000 * 1024 * 1024 * 1024 * 1024; // 1000 TB

  return new Response(finalYaml, {
    status: 200,
    headers: {
      'Content-Type': 'text/yaml; charset=utf-8',
      'Content-Disposition': `attachment; filename=Global-${proxyUsername}.yaml`,
      'Profile-Update-Interval': '24',
      'Subscription-Userinfo': `upload=${totalUp}; download=${totalDown}; total=${displayTotal}; expire=0`,
      'profile-title': `PNM Global - ${proxyUsername}`
    }
  });
}
