import './globals.css';

export const metadata = {
  title: 'PNM Panel — Proxy Node Manager',
  description: 'Multi-node proxy management dashboard',
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
