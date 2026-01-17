import { useState } from 'react'
import type { ResourceCost } from '../../types'
import { CostNode } from './CostNode'

interface Props {
    resources: ResourceCost[]
    onSelectSymbolic?: (resource: ResourceCost) => void
}

export function CostTree({ resources, onSelectSymbolic }: Props) {
    const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set())
    const [selectedNode, setSelectedNode] = useState<string | null>(null)

    const toggleNode = (address: string) => {
        const next = new Set(expandedNodes)
        if (next.has(address)) {
            next.delete(address)
        } else {
            next.add(address)
        }
        setExpandedNodes(next)
    }

    const handleNodeClick = (resource: ResourceCost) => {
        setSelectedNode(resource.address)
        if (resource.coverageType === 'symbolic' && onSelectSymbolic) {
            onSelectSymbolic(resource)
        }
    }

    // Group resources by cloud
    const byCloud = resources.reduce((acc, r) => {
        const cloud = r.type.split('_')[0] // aws_instance -> aws
        if (!acc[cloud]) acc[cloud] = []
        acc[cloud].push(r)
        return acc
    }, {} as Record<string, ResourceCost[]>)

    return (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
            <div className="p-4 border-b border-slate-200 bg-slate-50">
                <h2 className="text-lg font-semibold text-slate-900">Resource Cost Tree</h2>
                <p className="text-sm text-slate-500 mt-1">Click to expand • Hover for details</p>
            </div>

            <div className="divide-y divide-slate-100">
                {Object.entries(byCloud).map(([cloud, cloudResources]) => (
                    <div key={cloud}>
                        {/* Cloud header */}
                        <div className="px-4 py-2 bg-slate-50 text-sm font-medium text-slate-700 uppercase tracking-wide">
                            {cloud.toUpperCase()}
                            <span className="ml-2 text-slate-400">
                                ${cloudResources.reduce((sum, r) => sum + r.cost, 0).toFixed(2)}
                            </span>
                        </div>

                        {/* Resources */}
                        {cloudResources.map((resource) => (
                            <CostNode
                                key={resource.address}
                                resource={resource}
                                isExpanded={expandedNodes.has(resource.address)}
                                isSelected={selectedNode === resource.address}
                                onToggle={() => toggleNode(resource.address)}
                                onClick={() => handleNodeClick(resource)}
                            />
                        ))}
                    </div>
                ))}
            </div>
        </div>
    )
}
