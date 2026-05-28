import { describe, expect, it } from 'vitest'
import {
  builtinTouchesIface,
  IFACE_ALL,
  IFACE_FLOATING,
  ruleTouchesIface,
} from './firewallIface'

describe('ruleTouchesIface', () => {
  it('matches forward rule on iif or oif', () => {
    const r = { chain: 'forward', iif: 'eth0', oif: 'eth1' }
    expect(ruleTouchesIface(r, 'eth0', 'forward')).toBe(true)
    expect(ruleTouchesIface(r, 'eth1', 'forward')).toBe(true)
    expect(ruleTouchesIface(r, 'eth2', 'forward')).toBe(false)
  })

  it('floating input has no iif', () => {
    const r = { chain: 'input', iif: '' }
    expect(ruleTouchesIface(r, IFACE_FLOATING, 'input')).toBe(true)
    expect(ruleTouchesIface(r, 'eth0', 'input')).toBe(false)
  })

  it('all shows everything', () => {
    const r = { chain: 'input', iif: 'eth0' }
    expect(ruleTouchesIface(r, IFACE_ALL, 'input')).toBe(true)
  })
})

describe('builtinTouchesIface', () => {
  it('lan-wan on lan and wan tabs', () => {
    const br = { id: 'sys-lan-wan-eth1' }
    expect(builtinTouchesIface(br, 'br0', 'br0', 'eth1')).toBe(true)
    expect(builtinTouchesIface(br, 'eth1', 'br0', 'eth1')).toBe(true)
    expect(builtinTouchesIface(br, 'eth2', 'br0', 'eth1')).toBe(false)
  })

  it('ifb0 only on ifb0 tab', () => {
    const br = { id: 'sys-ifb0' }
    expect(builtinTouchesIface(br, 'ifb0', 'br0', 'eth1')).toBe(true)
    expect(builtinTouchesIface(br, 'br0', 'br0', 'eth1')).toBe(false)
  })
})
