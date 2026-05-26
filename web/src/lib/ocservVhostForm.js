/** Shared vhost form defaults and API payload helpers */

import { clientMbpsFromOcserv, ocservBpsFromClientMbps } from '@/lib/ocservRate'

export function emptyVhostForm() {
  return {
    enabled: true,
    domain: '',
    comment: '',
    auth_method: '',
    plain_passwd_path: '',
    radius: null,
    managed_cert_id: '',
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

/** 从全局 OCServ 配置生成新 vhost 表单（创建后可在高级页逐项修改） */
export function vhostFormFromGlobal(cfg) {
  const g = cfg || {}
  const adv = g.advanced || {}
  const auth = (g.auth_method || 'plain').trim()
  return normalizeVhostFromApi({
    enabled: true,
    domain: '',
    comment: '',
    auth_method: auth,
    plain_passwd_path: '',
    radius: null,
    server_cert_path: g.server_cert_path || '',
    server_key_path: g.server_key_path || '',
    ca_cert_path: g.ca_cert_path || '',
    ipv4_network: g.ipv4_network || '',
    ipv4_netmask: g.ipv4_netmask || '',
    dns: g.dns || [],
    routes: g.routes || [],
    no_routes: g.no_routes || [],
    config_per_group: g.config_per_group || adv.config_per_group || '',
    config_per_user: g.config_per_user || '',
    default_group_config: g.default_group_config || '',
    default_user_config: g.default_user_config || '',
    auto_select_group: !!g.auto_select_group,
    default_select_group: g.default_select_group || '',
    compression: !!adv.compression,
    dtls_legacy: !!adv.dtls_legacy,
    cisco_client_compat: !!adv.cisco_client_compat,
    cisco_svc_client_compat: !!adv.cisco_svc_client_compat,
    deny_roaming: !!adv.deny_roaming,
    predictable_ips: !!adv.predictable_ips,
    camouflage: !!adv.camouflage,
    camouflage_realm: adv.camouflage_realm || '',
    max_same_clients: adv.max_same_clients || 0,
    keepalive: adv.keepalive && adv.keepalive_sec > 0 ? adv.keepalive_sec : 0,
    dpd: adv.dpd && adv.dpd_sec > 0 ? adv.dpd_sec : 0,
    mobile_dpd: adv.mobile_dpd && adv.mobile_dpd_sec > 0 ? adv.mobile_dpd_sec : 0,
    cookie_timeout: adv.cookie_timeout || 0,
    rekey_time: adv.rekey && adv.rekey_time > 0 ? adv.rekey_time : 0,
    rekey_method: adv.rekey_method || '',
    ...ocservBpsFromClientMbps(
      clientMbpsFromOcserv(adv.rx_data_per_sec, adv.tx_data_per_sec).downMbps,
      clientMbpsFromOcserv(adv.rx_data_per_sec, adv.tx_data_per_sec).upMbps,
    ),
    default_domain: adv.default_domain || '',
    cert_user_oid: adv.cert_user_oid || '',
    tls_priorities: adv.tls_priorities || '',
    acct_enabled: !!(g.radius && g.radius.acct_enabled),
    stats_report_time: (g.radius && g.radius.stats_report_time) || 0,
  })
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
  delete body.users
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
