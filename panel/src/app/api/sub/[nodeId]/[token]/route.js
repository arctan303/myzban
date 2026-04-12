import { getDb } from '../../../../../lib/db';
import { nodeApi } from '../../../../../lib/nodeApi';
import { promises as dns } from 'dns';

export async function GET(request, { params }) {
  const { nodeId, token } = params;

  if (!nodeId || !token) {
    return new Response('Not Found', { status: 404 });
  }

  // 1. Get Node Info from Panel's local DB
  const db = getDb();
  const node = db.prepare('SELECT * FROM nodes WHERE id = ?').get(nodeId);

  if (!node) {
    return new Response('Node Not Found', { status: 404 });
  }

  try {
    // 2. Fetch User Info from Node using Admin Token (since the node doesn't expose public sub anymore)
    // We need a way to get the user by their sub_token.
    // The safest way is to fetch all users and find the one matching the token.
    // Since panel handles sub urls, if there are many users, a dedicated endpoint `/api/v1/users/token/{token}` would be better.
    // Let's just fetch all users for now since it's a small script.
    const users = await nodeApi(node.address, node.admin_token, '/api/v1/users');
    const user = users.find(u => u.sub_token === token);

    if (!user || (!user.enabled)) {
      return new Response('User Not Found or Disabled', { status: 404 });
    }

    if (user.traffic_limit > 0 && (user.traffic_up + user.traffic_down >= user.traffic_limit)) {
      return new Response('Traffic Limit Exceeded', { status: 403 });
    }

    // 3. Fetch Node's Proxy Configuration Status
    const status = await nodeApi(node.address, node.admin_token, '/api/v1/status');
    const nodeDetails = await nodeApi(node.address, node.admin_token, '/api/v1/node');

    let serverHost = new URL(node.address).hostname;
    let resolvedIp = serverHost;

    if ((node.use_reported_ip === 1 || node.use_reported_ip === undefined) && node.last_detected_ip) {
      resolvedIp = node.last_detected_ip;
    } else {
      try {
        const result = await dns.lookup(serverHost);
        if (result && result.address) {
          resolvedIp = result.address;
        }
      } catch (e) {
        console.error(`DNS lookup failed for ${serverHost}:`, e.message);
      }
    }

    // 4. Generate YAML
    let proxies = [];

    if (status.vless?.installed && status.vless?.running) {
      proxies.push(`  - name: "${node.name}-TCP"
    type: vless
    server: ${resolvedIp}
    port: ${status.vless.port}
    uuid: ${user.uuid}
    tls: true
    flow: xtls-rprx-vision
    network: tcp
    servername: ${nodeDetails.dest_domain || 'www.cloudflare.com'}
    client-fingerprint: chrome
    reality-opts:
      public-key: ${nodeDetails.xray_pub_key}
      short-id: ${nodeDetails.short_id || ''}`);
    }

    if (status.hysteria2?.installed && status.hysteria2?.running) {
      proxies.push(`  - name: "${node.name}-UDP"
    type: hysteria2
    server: ${resolvedIp}
    port: ${status.hysteria2.port}
    password: ${user.hy2_password}
    up: 50 Mbps
    down: 100 Mbps
    sni: ${nodeDetails.dest_domain || 'www.cloudflare.com'}
    skip-cert-verify: true
    alpn:
      - h3`);
    }

    if (proxies.length === 0) {
      return new Response('No proxy protocols running on this node', { status: 503 });
    }

    const proxyNames = [];
    if (status.vless?.installed && status.vless?.running) proxyNames.push(`"${node.name}-TCP"`);
    if (status.hysteria2?.installed && status.hysteria2?.running) proxyNames.push(`"${node.name}-UDP"`);

    const yaml = `# ProxyNode Manager - ${user.username}
# Generated from Panel for Node: ${node.name}

proxies:
${proxies.join('\n')}

proxy-groups:
  - name: PROXIES
    type: select
    proxies:
${proxyNames.map(n => `      - ${n}`).join('\n')}

rules:
  - MATCH,PROXIES
`;

    const displayTotal = user.traffic_limit > 0 ? user.traffic_limit : 1000 * 1024 * 1024 * 1024 * 1024;
    return new Response(yaml, {
      status: 200,
      headers: {
        'Content-Type': 'text/yaml; charset=utf-8',
        'Content-Disposition': `attachment; filename=Node-${node.name}-${user.username}.yaml`,
        'Profile-Update-Interval': '24',
        'Subscription-Userinfo': `upload=${user.traffic_up}; download=${user.traffic_down}; total=${displayTotal}; expire=0`,
        'profile-title': `PNM Node - ${node.name} - ${user.username}`
      }
    });

  } catch (error) {
    console.error('Subscription error:', error);
    return new Response('Internal Server Error fetching config from node', { status: 500 });
  }
}
