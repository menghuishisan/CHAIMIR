// NetworkAnimation 在登录布局中绘制低干扰节点连线背景，并尊重减少动态设置。
import React, { useEffect, useRef } from 'react'
import styles from './NetworkAnimation.module.css'

interface Node {
  x: number
  y: number
  vx: number
  vy: number
  radius: number
  color: string
}

export const NetworkAnimation: React.FC = () => {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  // 从设计令牌读取运行时颜色，避免在 Canvas 逻辑中复制品牌色值。
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const rootStyles = getComputedStyle(document.documentElement)
    const primaryColor = rootStyles.getPropertyValue('--color-accent').trim() || 'CanvasText'
    const secondaryColor = rootStyles.getPropertyValue('--color-secondary').trim() || 'CanvasText'
    const particleCount = 40
    const connectionDistance = 150
    let animationFrameId: number

    // 用户选择减少动态时只绘制静态网络，不持续更新坐标。
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches

    const resize = () => {
      if (!canvas.parentElement) return
      canvas.width = canvas.parentElement.clientWidth
      canvas.height = canvas.parentElement.clientHeight
    }

    window.addEventListener('resize', resize)
    resize()

    const nodes: Node[] = Array.from({ length: particleCount }).map(() => ({
      x: Math.random() * canvas.width,
      y: Math.random() * canvas.height,
      vx: (Math.random() - 0.5) * 1.2,
      vy: (Math.random() - 0.5) * 1.2,
      radius: Math.random() * 2 + 1.5,
      color: Math.random() > 0.8 ? secondaryColor : primaryColor,
    }))

    const draw = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height)

      // 节点距离越近连线越明显，保持背景可见但不干扰登录表单。
      for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
          const dx = nodes[i].x - nodes[j].x
          const dy = nodes[i].y - nodes[j].y
          const dist = Math.sqrt(dx * dx + dy * dy)

          if (dist < connectionDistance) {
            ctx.beginPath()
            ctx.strokeStyle = primaryColor
            ctx.globalAlpha = 1 - dist / connectionDistance
            ctx.lineWidth = 0.5
            ctx.moveTo(nodes[i].x, nodes[i].y)
            ctx.lineTo(nodes[j].x, nodes[j].y)
            ctx.stroke()
            ctx.globalAlpha = 1
          }
        }
      }

      // 节点使用主副双色令牌，和登录页品牌视觉保持一致。
      nodes.forEach(node => {
        ctx.beginPath()
        ctx.arc(node.x, node.y, node.radius, 0, Math.PI * 2)
        ctx.fillStyle = node.color
        ctx.shadowBlur = 10
        ctx.shadowColor = node.color
        ctx.fill()
        ctx.shadowBlur = 0
      })
    }

    const update = () => {
      if (prefersReducedMotion) {
        draw()
        return
      }

      nodes.forEach(node => {
        node.x += node.vx
        node.y += node.vy

        // 触达画布边缘后反弹，避免节点离开可视区域。
        if (node.x < 0 || node.x > canvas.width) node.vx *= -1
        if (node.y < 0 || node.y > canvas.height) node.vy *= -1
      })

      draw()
      animationFrameId = requestAnimationFrame(update)
    }

    update()

    return () => {
      window.removeEventListener('resize', resize)
      cancelAnimationFrame(animationFrameId)
    }
  }, [])

  return <canvas ref={canvasRef} className={styles.canvas} />
}
