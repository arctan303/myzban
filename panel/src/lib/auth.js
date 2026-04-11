const jwt = require('jsonwebtoken');

function getSecret() {
  // Use env var or fall back to a generated secret stored in the DB
  if (process.env.JWT_SECRET) return process.env.JWT_SECRET;

  const { getDb } = require('./db');
  const db = getDb();
  let row = db.prepare('SELECT value FROM settings WHERE key = ?').get('jwt_secret');
  if (!row) {
    const crypto = require('crypto');
    const secret = crypto.randomBytes(32).toString('hex');
    db.prepare('INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)').run('jwt_secret', secret);
    return secret;
  }
  return row.value;
}

function signToken(payload) {
  return jwt.sign(payload, getSecret(), { expiresIn: '7d' });
}

function verifyToken(token) {
  try {
    return jwt.verify(token, getSecret());
  } catch {
    return null;
  }
}

// Extract and verify token from Authorization header
function requireAuth(request) {
  const authHeader = request.headers.get('authorization');
  if (!authHeader || !authHeader.startsWith('Bearer ')) {
    return null;
  }
  const token = authHeader.slice(7);
  return verifyToken(token);
}

// Require admin role
function requireAdmin(request) {
  const user = requireAuth(request);
  if (!user || user.role !== 'admin') {
    return null;
  }
  return user;
}

module.exports = { signToken, verifyToken, requireAuth, requireAdmin };
