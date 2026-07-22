// download 文件集中维护浏览器 Blob 保存动作，页面只负责获取业务文件。

/** saveBlob 通过临时对象地址触发文件保存，并在同一任务周期后释放地址。 */
export function saveBlob(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = fileName
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}
