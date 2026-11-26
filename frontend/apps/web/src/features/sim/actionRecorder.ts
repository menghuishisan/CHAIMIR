import type { SimActionLog } from '@chaimir/api-client'

type NewSimAction = Omit<SimActionLog, 'seq' | 'created_at'>
type SavedSimAction = NewSimAction & Pick<SimActionLog, 'seq'>

export function createSimActionRecorder(
  lastSequence: number,
  persist: (action: SavedSimAction) => Promise<unknown>,
  onSaved: (action: SavedSimAction) => void,
  onFailure: (error: unknown, action: SavedSimAction) => void,
) {
  let nextSequence = lastSequence + 1
  let failed = false
  let pending = Promise.resolve()

  return (action: NewSimAction) => {
    if (failed) return
    pending = pending.then(async () => {
      if (failed) return
      const saved = { ...action, seq: nextSequence }
      try {
        await persist(saved)
        nextSequence += 1
      } catch (error) {
        failed = true
        onFailure(error, saved)
        return
      }
      onSaved(saved)
    })
  }
}
