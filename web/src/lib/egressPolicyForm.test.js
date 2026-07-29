import { describe, expect, it } from 'vitest'
import {
  buildEgressBody,
  emptyEgressForm,
  isAutoManagedEgress,
  optimalNoSnatForWan,
  pickOptimalWanLinkId,
  policyToEditForm,
} from './egressPolicyForm'

describe('pickOptimalWanLinkId', () => {
  const links = [
    { id: 'w1', device: 'eth0', enabled: true, policy_only: false },
    { id: 'w2', device: 'wg0', enabled: true, policy_only: true },
    { id: 'w3', device: 'eth1', enabled: false },
  ]

  it('prefers DEV_WAN device', () => {
    expect(pickOptimalWanLinkId(links, 'eth0')).toBe('w1')
  })

  it('prefers non-policy-only when DEV_WAN missing', () => {
    expect(pickOptimalWanLinkId(links, 'missing')).toBe('w1')
  })
})

describe('optimalNoSnatForWan', () => {
  it('true for policy-only / managed tunnels', () => {
    expect(optimalNoSnatForWan({ policy_only: true })).toBe(true)
    expect(optimalNoSnatForWan({ warp_managed: true })).toBe(true)
    expect(optimalNoSnatForWan({ iface_managed: true })).toBe(true)
    expect(optimalNoSnatForWan({ policy_only: false })).toBe(false)
  })
})

describe('buildEgressBody / policyToEditForm', () => {
  it('round-trips alias modes', () => {
    const form = emptyEgressForm({
      name: 't',
      wan_link_id: 'w1',
      src_mode: 'alias',
      src_alias: 'lan',
      dst_mode: 'cidr',
      dst_cidr: '1.2.3.0/24',
      no_snat: true,
      priority: 50,
      enabled: true,
    })
    const body = buildEgressBody(form)
    expect(body.src_alias).toBe('lan')
    expect(body.dst_cidr).toBe('1.2.3.0/24')
    expect(body.no_snat).toBe(true)
    expect(body.snat_ip).toBeUndefined()

    const back = policyToEditForm({
      name: 't',
      wan_link_id: 'w1',
      src_alias: 'lan',
      dst_cidr: '1.2.3.0/24',
      no_snat: true,
      priority: 50,
      enabled: true,
    })
    expect(back.src_mode).toBe('alias')
    expect(back.dst_mode).toBe('cidr')
  })
})

describe('isAutoManagedEgress', () => {
  it('detects auto prefixes', () => {
    expect(isAutoManagedEgress({ id: 'auto-iface-pr-1' })).toBe(true)
    expect(isAutoManagedEgress({ id: 'auto-fw-gw-x' })).toBe(true)
    expect(isAutoManagedEgress({ id: 'user-1' })).toBe(false)
  })
})
