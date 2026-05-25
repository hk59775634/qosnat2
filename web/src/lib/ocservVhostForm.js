/** Shared vhost form defaults and API payload helpers */

export function emptyVhostForm() {
  return {
    enabled: true,
    domain: '',
    comment: '',
    auth_method: '',
    plain_passwd_path: '',
    radius: null,
    server_cert_path: '',
    server_key_path: '',
    ca_cert_path: '',
    crl_path: '',
    dh_params_path: '',
    tls_priorities: '',
    cert_user_oid: '',
    cert_group_oid: '',
    ipv4_network: '',
    ipv4_netmask: '',
    ipv6_network: '',
    ipv6_prefix: 0,
    dns: [],
    nbns: [],
    default_domain: '',
    tunnel_all_dns: false,
    mtu: 0,
    routes: [],
    no_routes: [],
    iroutes: [],
    expose_iroutes: false,
    rx_data_per_sec: 0,
    tx_data_per_sec: 0,
    pkt_mtu_size: 0,
    idle_timeout: 0,
    session_timeout: 0,
    mobile_idle_timeout: 0,
    max_same_clients: 0,
    keepalive: 0,
    dpd: 0,
    mobile_dpd: 0,
    cookie_timeout: 0,
    deny_roaming: false,
    persistent_cookies: false,
    rekey_time: 0,
    rekey_method: '',
    compression: false,
    predictable_ips: false,
    dtls_legacy: false,
    cisco_client_compat: false,
    cisco_svc_client_compat: false,
    no_udp: false,
    banner: '',
    pre_login_banner: '',
    camouflage: false,
    camouflage_secret: '',
    camouflage_realm: '',
    config_per_user: '',
    config_per_group: '',
    default_user_config: '',
    default_group_config: '',
    select_groups: [],
    auto_select_group: false,
    default_select_group: '',
    connect_script: '',
    disconnect_script: '',
    acct_enabled: false,
    stats_report_time: 0,
  }
}

export function emptyVhostRadius() {
  return {
    server: '',
    auth_port: 1812,
    acct_port: 1813,
    secret: '',
    groupconfig: true,
    acct_enabled: false,
    stats_report_time: 360,
    nas_identifier: '',
    config_path: '',
  }
}

export function emptyBasicVhost() {
  return {
    enabled: true,
    domain: '',
    comment: '',
    auth_method: '',
  }
}

export function normalizeVhostFromApi(v) {
  const base = emptyVhostForm()
  return {
    ...base,
    ...v,
    dns: v.dns || [],
    nbns: v.nbns || [],
    routes: v.routes || [],
    no_routes: v.no_routes || [],
    iroutes: v.iroutes || [],
    select_groups: v.select_groups || [],
    radius: v.radius != null ? { ...emptyVhostRadius(), ...v.radius } : null,
  }
}

export function buildVhostPayload(form, { radiusSecret = '', camouflageSecret = '' } = {}) {
  const body = { ...form, domain: String(form.domain || '').trim() }
  if (body.radius && !String(body.radius.server || '').trim()) {
    body.radius = null
  }
  if (body.radius && radiusSecret) {
    body.radius = { ...body.radius, secret: radiusSecret }
  }
  if (camouflageSecret) {
    body.camouflage_secret = camouflageSecret
  }
  return body
}
