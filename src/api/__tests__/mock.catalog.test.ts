/** 店主维护资料：门店、化妆师、项目、作品 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { freshMock, useMockClock } from '@/test/helpers'
import type { ArtistInput, ServiceInput, WorkInput } from '@/api/types'

let api: Awaited<ReturnType<typeof freshMock>>['api']
let simulatePaid: Awaited<ReturnType<typeof freshMock>>['simulatePaid']

beforeEach(async () => {
  useMockClock()
  ;({ api, simulatePaid } = await freshMock())
})
afterEach(() => { vi.useRealTimers() })

const code = (e: unknown) => (e as { code?: string })?.code
const message = (e: unknown) => (e as { message?: string })?.message

const artist: ArtistInput = { name: '  小满 ', years: 3, avatar: 'placeholder:g2', title: '' }
const service: ServiceInput = {
  name: '毕业照妆', summary: '适合毕业照', category: 'camera', durationMin: 75, price: 19800, deposit: 3000,
  includes: ['底妆', ' ', '眼妆'], tags: ['上镜'], cover: 'placeholder:g6', images: ['placeholder:g6'],
}
const work: WorkInput = { title: '夏日毕业照', category: 'camera', artistId: 'a1', image: 'placeholder:g5', ratio: 1.3, durationText: '约 75 分钟' }

async function freeSlot(artistId: string, date: string, serviceId = 's1') {
  const s = (await api.listSlots(artistId, date, serviceId)).find(x => x.available)
  if (!s) throw new Error(`${artistId} ${date} 没有空时段`)
  return s.time
}

describe('门店信息', () => {
  it('保存后客人端读到新的', async () => {
    const shop = await api.getShop()
    const saved = await api.updateShop({ ...shop, phone: ' 138 0000 0000 ', openHours: '10:00–20:00' })
    expect(saved.phone).toBe('138 0000 0000')
    expect((await api.getShop()).openHours).toBe('10:00–20:00')
  })

  it('不符合规则时返回 UNKNOWN 和原因，原来的不变', async () => {
    const shop = await api.getShop()
    await expect(api.updateShop({ ...shop, name: ' ' })).rejects.toSatisfy(e => code(e) === 'UNKNOWN' && message(e) === '店名还没填')
    await expect(api.updateShop({ ...shop, phone: '打电话' })).rejects.toSatisfy(e => code(e) === 'UNKNOWN')
    await expect(api.updateShop({ ...shop, latitude: 0, longitude: 0 })).rejects.toSatisfy(e => message(e) === '地图位置还没选')
    expect((await api.getShop()).name).toBe(shop.name)
  })
})

describe('化妆师', () => {
  it('新建排在最后，去掉首尾空格和空的头衔，排班里也有 TA', async () => {
    const a = await api.createArtist(artist)
    expect(a.name).toBe('小满')
    expect(a).not.toHaveProperty('title')
    const list = await api.listArtists()
    expect(list[list.length - 1].id).toBe(a.id)
    const ds = await api.getDaySchedule((await api.listScheduleDates())[1])
    expect(ds.artists.map(x => x.id)).toContain(a.id)
    expect(ds.cells).toHaveLength(ds.times.length * ds.artists.length)
  })

  it('改名不影响已经下的单', async () => {
    const date = (await api.listBookableDates())[2]
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: await freeSlot('a1', date) })
    await api.updateArtist('a1', { name: '大鲸', years: 9, avatar: 'placeholder:g1' })
    expect((await api.listArtists()).find(a => a.id === 'a1')?.name).toBe('大鲸')
    expect((await api.getBooking(booking.id)).artistName).toBe('小鲸')
  })

  it('还有没结束的预约、名下有作品、只剩一位时不能删', async () => {
    const date = (await api.listBookableDates())[2]
    const a = await api.createArtist(artist)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: a.id, date, time: await freeSlot(a.id, date) })
    await expect(api.deleteArtist(a.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
    await api.cancelBooking(booking.id)

    const w = await api.createWork({ ...work, artistId: a.id })
    await expect(api.deleteArtist(a.id)).rejects.toSatisfy(e => message(e) === 'TA 名下还有 1 个作品，先改给别人或删掉')
    await api.deleteWork(w.id)
    await api.deleteArtist(a.id)
    expect((await api.listArtists()).map(x => x.id)).not.toContain(a.id)
  })

  it('不能删到一位都不剩', async () => {
    for (const w of await api.listOwnerWorks()) await api.deleteWork(w.id)
    await api.deleteArtist('a2')
    await api.deleteArtist('a3')
    await expect(api.deleteArtist('a1')).rejects.toSatisfy(e => message(e) === '至少要留一位化妆师')
  })

  it('没找到返回 NOT_FOUND', async () => {
    await expect(api.updateArtist('nope', artist)).rejects.toSatisfy(e => code(e) === 'NOT_FOUND')
  })
})

describe('项目', () => {
  it('新建从 0 人选择开始，空行去掉，封面是第一张图', async () => {
    const s = await api.createService({ ...service, cover: 'whatever' })
    expect(s.bookedCount).toBe(0)
    expect(s.includes).toEqual(['底妆', '眼妆'])
    expect(s.cover).toBe('placeholder:g6')
    expect((await api.listServices()).map(x => x.id)).toContain(s.id)
  })

  it('定金不能比价格高，时长按 15 分钟一档', async () => {
    await expect(api.createService({ ...service, deposit: 20000 })).rejects.toSatisfy(e => message(e) === '定金不能比价格高')
    await expect(api.createService({ ...service, durationMin: 70 })).rejects.toSatisfy(e => code(e) === 'UNKNOWN')
  })

  it('改价只影响之后的新预约', async () => {
    const date = (await api.listBookableDates())[2]
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: await freeSlot('a1', date) })
    const s1 = (await api.listOwnerServices()).find(s => s.id === 's1')!
    const { id: _id, bookedCount: _n, ...input } = s1
    const updated = await api.updateService('s1', { ...input, price: 32800 })
    expect(updated.bookedCount).toBe(s1.bookedCount)
    expect((await api.getBooking(booking.id)).price).toBe(29800)
    const { booking: b2 } = await api.createBooking({ serviceId: 's1', artistId: 'a2', date, time: await freeSlot('a2', date) })
    expect(b2.price).toBe(32800)
  })

  it('下架后客人看不到、不能新约，详情还能打开；已经约了的照常进行', async () => {
    const date = (await api.listBookableDates())[2]
    const { booking } = await api.createBooking({ serviceId: 's2', artistId: 'a1', date, time: await freeSlot('a1', date) })
    await simulatePaid(booking.id)
    const s2 = (await api.listOwnerServices()).find(s => s.id === 's2')!
    const { id: _id, bookedCount: _n, ...input } = s2
    await api.updateService('s2', { ...input, hidden: true })

    expect((await api.listServices()).map(s => s.id)).not.toContain('s2')
    expect((await api.getService('s2')).hidden).toBe(true)
    expect((await api.listOwnerServices()).find(s => s.id === 's2')?.hidden).toBe(true)
    await expect(api.createBooking({ serviceId: 's2', artistId: 'a2', date, time: await freeSlot('a2', date) }))
      .rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
    expect((await api.getBooking(booking.id)).status).toBe('pending_confirm')

    // 作品的“预约同款”不再预选这个项目，店主那边照常能看到关联
    expect((await api.listWorks()).find(w => w.id === 'w2')).not.toHaveProperty('serviceId')
    expect((await api.listOwnerWorks()).find(w => w.id === 'w2')?.serviceId).toBe('s2')
  })

  it('不能把最后一个在接预约的项目下架或删掉', async () => {
    const list = await api.listOwnerServices()
    for (const s of list.slice(1)) {
      const { id, bookedCount: _n, ...input } = s
      await api.updateService(id, { ...input, hidden: true })
    }
    const { id, bookedCount: _n, ...input } = list[0]
    await expect(api.updateService(id, { ...input, hidden: true })).rejects.toSatisfy(e => message(e) === '至少要留一个在接预约的项目')
    await expect(api.deleteService(id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })

  it('有没结束的预约时不能删；删掉后作品不再关联它', async () => {
    const date = (await api.listBookableDates())[2]
    const { booking } = await api.createBooking({ serviceId: 's2', artistId: 'a1', date, time: await freeSlot('a1', date) })
    await expect(api.deleteService('s2')).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
    await api.cancelBooking(booking.id)
    await api.deleteService('s2')
    expect((await api.listOwnerServices()).map(s => s.id)).not.toContain('s2')
    expect((await api.listOwnerWorks()).filter(w => w.serviceId === 's2')).toEqual([])
  })
})

describe('作品', () => {
  it('新建排在最前，按分类筛选能看到', async () => {
    const w = await api.createWork({ ...work, serviceId: 's1' })
    expect((await api.listWorks())[0].id).toBe(w.id)
    expect((await api.listWorks('camera'))[0].id).toBe(w.id)
    expect((await api.listWorks('bridal')).map(x => x.id)).not.toContain(w.id)
  })

  it('化妆师或项目不存在时不能保存', async () => {
    await expect(api.createWork({ ...work, artistId: 'nope' })).rejects.toSatisfy(e => code(e) === 'UNKNOWN')
    await expect(api.createWork({ ...work, serviceId: 'nope' })).rejects.toSatisfy(e => code(e) === 'UNKNOWN')
  })

  it('修改和删除', async () => {
    const updated = await api.updateWork('w1', { ...work, title: '改个名' })
    expect(updated).toMatchObject({ id: 'w1', title: '改个名' })
    expect(updated).not.toHaveProperty('serviceId')
    await api.deleteWork('w1')
    expect((await api.listOwnerWorks()).map(w => w.id)).not.toContain('w1')
    await expect(api.deleteWork('w1')).rejects.toSatisfy(e => code(e) === 'NOT_FOUND')
  })
})
