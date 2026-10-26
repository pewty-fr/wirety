/**
 * An in-memory Wirety API for the front's integration tests (MSW handlers).
 *
 * It mirrors the server's REST contract closely enough for the pages and
 * modals to work end to end: create/update/delete persist in `db`, and every
 * mutating request is recorded in `requests` so tests can assert payloads.
 */
import { http, HttpResponse } from 'msw';
import type { AuthConfig } from '../contexts/AuthContext';
import type {
  APIToken,
  DNSMapping,
  Group,
  IPAMAllocation,
  Network,
  Peer,
  Policy,
  PolicyRule,
  Route,
  User,
} from '../types';

const API = '*/api/v1';

export interface RecordedRequest {
  method: string;
  path: string;
  body: unknown;
}

interface Db {
  /** False: GET /users/me answers 401 (signed out). */
  signedIn: boolean;
  authConfig: AuthConfig;
  me: User;
  users: User[];
  networks: Network[];
  peers: (Peer & { network_id: string })[];
  groups: Group[];
  policies: Policy[];
  routes: Route[];
  dns: DNSMapping[];
  tokens: APIToken[];
  defaults: { default_role: 'administrator' | 'user'; default_authorized_networks: string[] };
}

let seq = 0;
const newId = (prefix: string) => `${prefix}-${++seq}`;
const now = () => '2026-10-01T10:00:00Z';

export const db: Db = emptyDb();
export const requests: RecordedRequest[] = [];

function emptyDb(): Db {
  const admin: User = {
    id: 'admin',
    email: 'admin@wirety.local',
    name: 'Administrator',
    role: 'administrator',
    authorized_networks: [],
    created_at: now(),
    updated_at: now(),
    last_login_at: now(),
  };
  return {
    signedIn: true,
    authConfig: {
      enabled: false,
      issuer_url: '',
      client_id: '',
      simple_auth: true,
      authorization_endpoint: '',
      end_session_endpoint: '',
      scope: '',
      authorization_extra_params: '',
    },
    me: admin,
    users: [admin],
    networks: [],
    peers: [],
    groups: [],
    policies: [],
    routes: [],
    dns: [],
    tokens: [],
    defaults: { default_role: 'user', default_authorized_networks: [] },
  };
}

/** Resets the fake backend to an empty network list, logged in as admin. */
export function resetDb() {
  Object.assign(db, emptyDb());
  requests.length = 0;
  seq = 0;
}

/** Requests received for a method and path (e.g. 'POST', /\/groups$/). */
export function requestsTo(method: string, path: RegExp): RecordedRequest[] {
  return requests.filter((r) => r.method === method && path.test(r.path));
}

// ---- seed helpers -------------------------------------------------------------

export function seedNetwork(over: Partial<Network> = {}): Network {
  const n: Network = {
    id: newId('net'),
    name: 'corp',
    cidr: '10.42.0.0/24',
    dns: [],
    domain_suffix: 'internal',
    default_group_ids: [],
    created_at: now(),
    updated_at: now(),
    ...over,
  };
  db.networks.push(n);
  return n;
}

export function seedPeer(networkId: string, over: Partial<Peer> = {}): Peer {
  const index = db.peers.filter((p) => p.network_id === networkId).length + 2;
  const p = {
    id: newId('peer'),
    name: `peer-${index}`,
    public_key: `pubkey-${index}=`,
    address: `10.42.0.${index}`,
    endpoint: '',
    is_jump: false,
    use_agent: false,
    group_ids: [],
    created_at: now(),
    updated_at: now(),
    ...over,
    network_id: networkId,
  };
  db.peers.push(p);
  return p;
}

export function seedGroup(networkId: string, over: Partial<Group> = {}): Group {
  const g: Group = {
    id: newId('grp'),
    network_id: networkId,
    name: 'engineering',
    description: '',
    priority: 100,
    peer_ids: [],
    policy_ids: [],
    route_ids: [],
    created_at: now(),
    updated_at: now(),
    ...over,
  };
  db.groups.push(g);
  return g;
}

export function seedPolicy(networkId: string, over: Partial<Policy> = {}): Policy {
  const p: Policy = {
    id: newId('pol'),
    network_id: networkId,
    name: 'allow-web',
    description: '',
    rules: [],
    created_at: now(),
    updated_at: now(),
    ...over,
  };
  db.policies.push(p);
  return p;
}

export function seedRoute(networkId: string, over: Partial<Route> = {}): Route {
  const r: Route = {
    id: newId('rt'),
    network_id: networkId,
    name: 'datacenter',
    description: '',
    destination_cidr: '10.0.0.0/16',
    jump_peer_id: '',
    domain_suffix: 'internal',
    created_at: now(),
    updated_at: now(),
    ...over,
  };
  db.routes.push(r);
  return r;
}

export function seedDNS(routeId: string, over: Partial<DNSMapping> = {}): DNSMapping {
  const d: DNSMapping = {
    id: newId('dns'),
    route_id: routeId,
    name: 'nas',
    ip_address: '10.0.0.50',
    created_at: now(),
    updated_at: now(),
    ...over,
  };
  db.dns.push(d);
  return d;
}

export function seedUser(over: Partial<User> = {}): User {
  const u: User = {
    id: newId('user'),
    email: 'bob@example.com',
    name: 'Bob',
    role: 'user',
    authorized_networks: [],
    created_at: now(),
    updated_at: now(),
    last_login_at: now(),
    ...over,
  };
  db.users.push(u);
  return u;
}

// ---- helpers ------------------------------------------------------------------

const notFound = () => HttpResponse.json({ error: 'not found' }, { status: 404 });

function paginate<T>(items: T[], url: URL) {
  const page = Number(url.searchParams.get('page') ?? 1);
  const pageSize = Number(url.searchParams.get('page_size') ?? 20);
  return { data: items.slice((page - 1) * pageSize, page * pageSize), total: items.length, page, page_size: pageSize };
}

async function record(request: Request) {
  const path = new URL(request.url).pathname.replace(/^\/api\/v1/, '');
  let body: unknown = undefined;
  if (request.method !== 'GET' && request.method !== 'DELETE') {
    const text = await request.clone().text();
    body = text ? JSON.parse(text) : undefined;
  }
  requests.push({ method: request.method, path, body });
  return body as Record<string, unknown>;
}

function withNetworkName<T extends { network_id: string }>(item: T): T & { network_name?: string } {
  return { ...item, network_name: db.networks.find((n) => n.id === item.network_id)?.name };
}

/** The addresses given to the peers, as GET /ipam lists them. */
function allocations(): IPAMAllocation[] {
  return db.peers.flatMap((p) => {
    const n = db.networks.find((x) => x.id === p.network_id);
    if (!n) return [];
    const base = { network_id: n.id, network_name: n.name, peer_id: p.id, peer_name: p.name, allocated: true };
    return [
      ...(p.address && n.cidr ? [{ ...base, network_cidr: n.cidr, family: 'ipv4' as const, ip: p.address }] : []),
      ...(p.address_v6 && n.cidr_v6 ? [{ ...base, network_cidr: n.cidr_v6, family: 'ipv6' as const, ip: p.address_v6 }] : []),
    ];
  });
}

function groupsFor(peerId: string) {
  return db.groups.filter((g) => g.peer_ids.includes(peerId)).map((g) => g.id);
}

// ---- handlers -----------------------------------------------------------------

export const handlers = [
  // auth
  http.get(`${API}/auth/config`, () => HttpResponse.json(db.authConfig)),
  http.post(`${API}/auth/login`, async ({ request }) => {
    await record(request);
    db.signedIn = true;
    return HttpResponse.json({ session_hash: 'session', expires_in: 3600 });
  }),
  http.post(`${API}/auth/logout`, async ({ request }) => {
    await record(request);
    db.signedIn = false;
    return HttpResponse.json({});
  }),

  // users
  http.get(`${API}/users/me`, () =>
    db.signedIn ? HttpResponse.json(db.me) : HttpResponse.json({ error: 'unauthorized' }, { status: 401 }),
  ),
  http.put(`${API}/users/me`, async ({ request }) => {
    const body = await record(request);
    Object.assign(db.me, body);
    return HttpResponse.json(db.me);
  }),
  http.get(`${API}/users/me/tokens`, () => HttpResponse.json(db.tokens)),
  http.post(`${API}/users/me/tokens`, async ({ request }) => {
    const body = await record(request);
    const t: APIToken = { id: newId('tok'), name: String(body.name), created_at: now(), expires_at: body.expires_at as string | undefined };
    db.tokens.push(t);
    return HttpResponse.json({ ...t, token: 'wirety_tok_secret' }, { status: 201 });
  }),
  http.delete(`${API}/users/me/tokens/:id`, async ({ request, params }) => {
    await record(request);
    db.tokens = db.tokens.filter((t) => t.id !== params.id);
    return new HttpResponse(null, { status: 204 });
  }),
  http.get(`${API}/users/defaults`, () => HttpResponse.json(db.defaults)),
  http.put(`${API}/users/defaults`, async ({ request }) => {
    const body = await record(request);
    Object.assign(db.defaults, body);
    return HttpResponse.json(db.defaults);
  }),
  http.get(`${API}/users`, () => HttpResponse.json(db.users)),
  http.get(`${API}/users/:id`, ({ params }) => {
    const u = db.users.find((x) => x.id === params.id);
    return u ? HttpResponse.json(u) : notFound();
  }),
  http.post(`${API}/users`, async ({ request }) => {
    const body = await record(request);
    if (db.users.some((u) => u.email === body.email)) {
      return HttpResponse.json({ error: 'a user with this email already exists' }, { status: 409 });
    }
    const u = seedUser({
      email: String(body.email),
      name: String(body.name),
      role: body.role as User['role'],
      authorized_networks: (body.authorized_networks as string[]) ?? [],
    });
    return HttpResponse.json(u, { status: 201 });
  }),
  http.put(`${API}/users/:id`, async ({ request, params }) => {
    const body = await record(request);
    const u = db.users.find((x) => x.id === params.id);
    if (!u) return notFound();
    const { password: _password, ...rest } = body;
    void _password;
    Object.assign(u, rest);
    return HttpResponse.json(u);
  }),
  http.delete(`${API}/users/:id`, async ({ request, params }) => {
    await record(request);
    db.users = db.users.filter((u) => u.id !== params.id);
    return new HttpResponse(null, { status: 204 });
  }),

  // networks
  http.get(`${API}/networks`, ({ request }) => {
    const url = new URL(request.url);
    const filter = url.searchParams.get('filter')?.toLowerCase();
    const list = db.networks
      .filter((n) => !filter || n.name.toLowerCase().includes(filter))
      .map((n) => ({ ...n, peer_count: db.peers.filter((p) => p.network_id === n.id).length }));
    return HttpResponse.json(paginate(list, url));
  }),
  http.get(`${API}/networks/:id`, ({ params }) => {
    const n = db.networks.find((x) => x.id === params.id);
    return n ? HttpResponse.json({ ...n, peer_count: db.peers.filter((p) => p.network_id === n.id).length }) : notFound();
  }),
  http.post(`${API}/networks`, async ({ request }) => {
    const body = await record(request);
    if (db.networks.some((n) => n.name === body.name)) {
      return HttpResponse.json({ error: 'a network with this name already exists' }, { status: 409 });
    }
    const n = seedNetwork({
      name: String(body.name),
      cidr: String(body.cidr ?? ''),
      cidr_v6: body.cidr_v6 as string | undefined,
      dns: (body.dns as string[]) ?? [],
      domain_suffix: (body.domain_suffix as string) || 'internal',
      default_group_ids: (body.default_group_ids as string[]) ?? [],
    });
    return HttpResponse.json(n, { status: 201 });
  }),
  http.put(`${API}/networks/:id`, async ({ request, params }) => {
    const body = await record(request);
    const n = db.networks.find((x) => x.id === params.id);
    if (!n) return notFound();
    Object.assign(n, body);
    return HttpResponse.json(n);
  }),
  http.delete(`${API}/networks/:id`, async ({ request, params }) => {
    await record(request);
    db.networks = db.networks.filter((n) => n.id !== params.id);
    return new HttpResponse(null, { status: 204 });
  }),
  http.get(`${API}/networks/:id/acl`, () =>
    HttpResponse.json({ id: 'acl', name: 'acl', enabled: false, blocked_peers: {}, rules: [] }),
  ),

  // peers
  http.get(`${API}/networks/:id/peers`, ({ request, params }) => {
    const list = db.peers.filter((p) => p.network_id === params.id).map((p) => ({ ...p, group_ids: groupsFor(p.id) }));
    return HttpResponse.json(paginate(list, new URL(request.url)));
  }),
  http.get(`${API}/networks/:id/peers/:peerId`, ({ params }) => {
    const p = db.peers.find((x) => x.id === params.peerId);
    return p ? HttpResponse.json({ ...p, group_ids: groupsFor(p.id) }) : notFound();
  }),
  http.post(`${API}/networks/:id/peers`, async ({ request, params }) => {
    const body = await record(request);
    if (db.peers.some((p) => p.network_id === params.id && p.name === body.name)) {
      return HttpResponse.json({ error: 'a peer with this name already exists in the network' }, { status: 409 });
    }
    const p = seedPeer(String(params.id), {
      name: String(body.name),
      endpoint: (body.endpoint as string) ?? '',
      listen_port: body.listen_port as number | undefined,
      is_jump: Boolean(body.is_jump),
      use_agent: Boolean(body.use_agent),
      owner_id: body.owner_id as string | undefined,
      token: body.use_agent || body.is_jump ? 'enroll-token' : undefined,
    });
    return HttpResponse.json(p, { status: 201 });
  }),
  http.put(`${API}/networks/:id/peers/:peerId`, async ({ request, params }) => {
    const body = await record(request);
    const p = db.peers.find((x) => x.id === params.peerId);
    if (!p) return notFound();
    Object.assign(p, body);
    return HttpResponse.json(p);
  }),
  http.delete(`${API}/networks/:id/peers/:peerId`, async ({ request, params }) => {
    await record(request);
    db.peers = db.peers.filter((p) => p.id !== params.peerId);
    return new HttpResponse(null, { status: 204 });
  }),
  http.get(`${API}/networks/:id/peers/:peerId/session`, ({ params }) =>
    HttpResponse.json({ peer_id: params.peerId, has_active_agent: false, last_checked: now(), captive_portal_state: '' }),
  ),
  http.get(`${API}/networks/:id/peers/:peerId/config`, () =>
    HttpResponse.json({ config: '[Interface]\nPrivateKey = test\nAddress = 10.42.0.2/32\n' }),
  ),
  http.get(`${API}/networks/:id/peers/:peerId/reachability`, ({ params }) => {
    const p = db.peers.find((x) => x.id === params.peerId);
    return HttpResponse.json({
      peer_id: params.peerId,
      peer_name: p?.name ?? '',
      peer_address: p?.address ?? '',
      is_jump: p?.is_jump ?? false,
      peer_access: [],
      rules: [],
      routes: [],
    });
  }),
  http.get(`${API}/networks/:id/peers/:peerId/captive-portal-events`, () => HttpResponse.json([])),
  http.post(`${API}/networks/:id/peers/:peerId/revoke-auth`, async ({ request }) => {
    await record(request);
    return new HttpResponse(null, { status: 204 });
  }),

  // groups
  http.get(`${API}/networks/:id/groups`, ({ params }) =>
    HttpResponse.json(db.groups.filter((g) => g.network_id === params.id).map(withNetworkName)),
  ),
  http.get(`${API}/networks/:id/groups/:groupId`, ({ params }) => {
    const g = db.groups.find((x) => x.id === params.groupId);
    return g ? HttpResponse.json(withNetworkName(g)) : notFound();
  }),
  http.post(`${API}/networks/:id/groups`, async ({ request, params }) => {
    const body = await record(request);
    if (db.groups.some((g) => g.network_id === params.id && g.name === body.name)) {
      return HttpResponse.json({ error: 'a group with this name already exists' }, { status: 409 });
    }
    const g = seedGroup(String(params.id), {
      name: String(body.name),
      description: (body.description as string) ?? '',
      priority: (body.priority as number) ?? 100,
    });
    return HttpResponse.json(g, { status: 201 });
  }),
  http.put(`${API}/networks/:id/groups/:groupId`, async ({ request, params }) => {
    const body = await record(request);
    const g = db.groups.find((x) => x.id === params.groupId);
    if (!g) return notFound();
    Object.assign(g, body);
    return HttpResponse.json(g);
  }),
  http.delete(`${API}/networks/:id/groups/:groupId`, async ({ request, params }) => {
    await record(request);
    db.groups = db.groups.filter((g) => g.id !== params.groupId);
    return new HttpResponse(null, { status: 204 });
  }),
  ...(['peers', 'policies', 'routes'] as const).flatMap((kind) => {
    const key = ({ peers: 'peer_ids', policies: 'policy_ids', routes: 'route_ids' } as const)[kind];
    return [
      http.post(`${API}/networks/:id/groups/:groupId/${kind}/:itemId`, async ({ request, params }) => {
        await record(request);
        const g = db.groups.find((x) => x.id === params.groupId);
        if (!g) return notFound();
        if (!g[key].includes(String(params.itemId))) g[key].push(String(params.itemId));
        return new HttpResponse(null, { status: 204 });
      }),
      http.delete(`${API}/networks/:id/groups/:groupId/${kind}/:itemId`, async ({ request, params }) => {
        await record(request);
        const g = db.groups.find((x) => x.id === params.groupId);
        if (!g) return notFound();
        g[key] = g[key].filter((x) => x !== params.itemId);
        return new HttpResponse(null, { status: 204 });
      }),
    ];
  }),
  http.get(`${API}/networks/:id/groups/:groupId/policies`, ({ params }) => {
    const g = db.groups.find((x) => x.id === params.groupId);
    return HttpResponse.json(g ? g.policy_ids.map((id) => db.policies.find((p) => p.id === id)).filter(Boolean) : []);
  }),
  http.put(`${API}/networks/:id/groups/:groupId/policies/order`, async ({ request, params }) => {
    const body = await record(request);
    const g = db.groups.find((x) => x.id === params.groupId);
    if (g) g.policy_ids = body.policy_ids as string[];
    return new HttpResponse(null, { status: 204 });
  }),
  http.get(`${API}/networks/:id/groups/:groupId/routes`, ({ params }) => {
    const g = db.groups.find((x) => x.id === params.groupId);
    return HttpResponse.json(g ? g.route_ids.map((id) => db.routes.find((r) => r.id === id)).filter(Boolean) : []);
  }),

  // policies
  http.get(`${API}/networks/:id/policies`, ({ params }) =>
    HttpResponse.json(db.policies.filter((p) => p.network_id === params.id).map(withNetworkName)),
  ),
  http.get(`${API}/networks/:id/policies/:policyId`, ({ params }) => {
    const p = db.policies.find((x) => x.id === params.policyId);
    return p ? HttpResponse.json(withNetworkName(p)) : notFound();
  }),
  http.post(`${API}/networks/:id/policies`, async ({ request, params }) => {
    const body = await record(request);
    if (db.policies.some((p) => p.network_id === params.id && p.name === body.name)) {
      return HttpResponse.json({ error: 'a policy with this name already exists' }, { status: 409 });
    }
    const rules = ((body.rules as PolicyRule[]) ?? []).map((r) => ({ ...r, id: newId('rule') }));
    const p = seedPolicy(String(params.id), {
      name: String(body.name),
      description: (body.description as string) ?? '',
      rules,
    });
    return HttpResponse.json(p, { status: 201 });
  }),
  http.put(`${API}/networks/:id/policies/:policyId`, async ({ request, params }) => {
    const body = await record(request);
    const p = db.policies.find((x) => x.id === params.policyId);
    if (!p) return notFound();
    Object.assign(p, body);
    return HttpResponse.json(p);
  }),
  http.delete(`${API}/networks/:id/policies/:policyId`, async ({ request, params }) => {
    await record(request);
    db.policies = db.policies.filter((p) => p.id !== params.policyId);
    return new HttpResponse(null, { status: 204 });
  }),
  http.post(`${API}/networks/:id/policies/:policyId/rules`, async ({ request, params }) => {
    const body = await record(request);
    const p = db.policies.find((x) => x.id === params.policyId);
    if (!p) return notFound();
    const rule = { ...(body as unknown as PolicyRule), id: newId('rule') };
    p.rules.push(rule);
    return HttpResponse.json(rule, { status: 201 });
  }),
  http.delete(`${API}/networks/:id/policies/:policyId/rules/:ruleId`, async ({ request, params }) => {
    await record(request);
    const p = db.policies.find((x) => x.id === params.policyId);
    if (p) p.rules = p.rules.filter((r) => r.id !== params.ruleId);
    return new HttpResponse(null, { status: 204 });
  }),

  // routes and DNS
  http.get(`${API}/networks/:id/routes`, ({ params }) =>
    HttpResponse.json(db.routes.filter((r) => r.network_id === params.id).map(withNetworkName)),
  ),
  http.get(`${API}/networks/:id/routes/:routeId`, ({ params }) => {
    const r = db.routes.find((x) => x.id === params.routeId);
    return r ? HttpResponse.json(withNetworkName(r)) : notFound();
  }),
  http.post(`${API}/networks/:id/routes`, async ({ request, params }) => {
    const body = await record(request);
    const r = seedRoute(String(params.id), body as Partial<Route>);
    return HttpResponse.json(r, { status: 201 });
  }),
  http.put(`${API}/networks/:id/routes/:routeId`, async ({ request, params }) => {
    const body = await record(request);
    const r = db.routes.find((x) => x.id === params.routeId);
    if (!r) return notFound();
    // A field sent empty is cleared; one destination must remain.
    if (!{ ...r, ...body }.destination_cidr && !{ ...r, ...body }.destination_cidr_v6) {
      return HttpResponse.json({ error: 'at least one of destination_cidr or destination_cidr_v6 must remain set' }, { status: 400 });
    }
    Object.assign(r, body);
    return HttpResponse.json(r);
  }),
  http.delete(`${API}/networks/:id/routes/:routeId`, async ({ request, params }) => {
    await record(request);
    db.routes = db.routes.filter((r) => r.id !== params.routeId);
    return new HttpResponse(null, { status: 204 });
  }),
  http.get(`${API}/networks/:id/routes/:routeId/dns`, ({ params }) =>
    HttpResponse.json(db.dns.filter((d) => d.route_id === params.routeId)),
  ),
  http.post(`${API}/networks/:id/routes/:routeId/dns`, async ({ request, params }) => {
    const body = await record(request);
    const d: DNSMapping = {
      id: newId('dns'),
      route_id: String(params.routeId),
      name: String(body.name),
      ip_address: body.ip_address as string | undefined,
      ip_address_v6: body.ip_address_v6 as string | undefined,
      created_at: now(),
      updated_at: now(),
    };
    db.dns.push(d);
    return HttpResponse.json(d, { status: 201 });
  }),
  http.put(`${API}/networks/:id/routes/:routeId/dns/:dnsId`, async ({ request, params }) => {
    const body = await record(request);
    const d = db.dns.find((x) => x.id === params.dnsId);
    if (!d) return notFound();
    // A field sent empty is cleared; one address must remain.
    if (!{ ...d, ...body }.ip_address && !{ ...d, ...body }.ip_address_v6) {
      return HttpResponse.json({ error: 'at least one of ip_address or ip_address_v6 must remain set' }, { status: 400 });
    }
    Object.assign(d, body);
    return HttpResponse.json(d);
  }),
  http.delete(`${API}/networks/:id/routes/:routeId/dns/:dnsId`, async ({ request, params }) => {
    await record(request);
    db.dns = db.dns.filter((d) => d.id !== params.dnsId);
    return new HttpResponse(null, { status: 204 });
  }),

  // IPAM
  http.get(`${API}/ipam/available-cidrs`, () =>
    HttpResponse.json({
      base_cidr: '10.0.0.0/8',
      requested_max_peers: 50,
      suggested_prefix: 26,
      usable_hosts: 62,
      cidrs: ['10.1.0.0/26', '10.1.0.64/26', '10.1.0.128/26'],
    }),
  ),
  http.get(`${API}/ipam/networks/:id`, ({ params }) =>
    HttpResponse.json(allocations().filter((a) => a.network_id === params.id)),
  ),
  http.get(`${API}/ipam`, ({ request }) => {
    const url = new URL(request.url);
    const filter = url.searchParams.get('filter')?.toLowerCase();
    const list = allocations().filter(
      (a) => !filter || [a.network_name, a.ip, a.peer_name ?? ''].some((v) => v.toLowerCase().includes(filter)),
    );
    return HttpResponse.json(paginate(list, url));
  }),
];
