// TreePatternRenderer 渲染树结构和证明路径。

import React from 'react';
import { clsx } from 'clsx';
import type { FrameFocus, TreeNode, TreePattern } from '../../types';
import { PatternHeader } from '../PatternChrome';
import { elementVisualClasses, selectableElementProps } from '../patternUtils';
import './TreePatternRenderer.css';

/**
 * 渲染树结构及高亮路径,用于 Merkle 树、状态树和证明路径场景。
 */
export function TreePatternRenderer({
  pattern,
  focus,
  selectedElementId,
  onSelectElement,
}: {
  pattern: TreePattern;
  focus?: FrameFocus;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const nodeCount = countTreeNodes(pattern.data.root);
  return (
    <section className="sim-pattern sim-pattern--tree" aria-label={pattern.title}>
      <PatternHeader mode="tree" title={pattern.title} meta={`${nodeCount} 节点 / 路径 ${pattern.data.highlightedPath.length} 层`} />
      <div className="sim-tree">{renderTreeNode(pattern.data.root, pattern.data.highlightedPath, selectedElementId, onSelectElement, focus, true)}</div>
    </section>
  );
}

/**
 * 递归渲染树节点,保持父子结构与证明路径关系可见。
 */
function renderTreeNode(
  node: TreeNode,
  path: string[],
  selectedElementId?: string,
  onSelectElement?: (elementId: string, elementType?: string) => void,
  focus?: FrameFocus,
  isRoot = false
): React.ReactElement {
  const pathParent = node.children?.some((child) => treeContainsPath(child, path)) ?? false;
  return (
    <div className={clsx('sim-tree__node', isRoot && 'is-root', path.includes(node.id) && 'is-highlighted', pathParent && 'is-path-parent', elementVisualClasses(node, focus), selectedElementId === node.id && 'is-selected')}>
      <div className="sim-tree__box" {...selectableElementProps(node.id, onSelectElement, 'tree-node')}>
        <span>{node.label}</span>
        <code>{node.hash.slice(0, 10)}</code>
      </div>
      {node.children && node.children.length > 0 && (
        <div className="sim-tree__children">
          {node.children.map((child) => renderTreeNode(child, path, selectedElementId, onSelectElement, focus))}
        </div>
      )}
    </div>
  );
}

/**
 * countTreeNodes 统计树节点数量,用于 Merkle/Trie 类结构给出规模感。
 */
function countTreeNodes(node: TreeNode): number {
  return 1 + (node.children?.reduce((sum, child) => sum + countTreeNodes(child), 0) ?? 0);
}

/**
 * treeContainsPath 判断当前子树是否包含高亮路径节点,用于标出 proof path 的父链。
 */
function treeContainsPath(node: TreeNode, path: string[]): boolean {
  return path.includes(node.id) || Boolean(node.children?.some((child) => treeContainsPath(child, path)));
}
