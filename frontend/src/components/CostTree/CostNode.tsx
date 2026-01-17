import type { ResourceCost, CostComponent } from '../../types'

interface Props {
    resource: ResourceCost
    isExpanded: boolean
    isSelected: boolean
    onToggle: () => void
    onClick: () => void
}

const COVERAGE_ICONS = {
    numeric: '🟢',
    symbolic: '🟡',
    indirect: '⚪',
    unsupported: '🔴',
}

const COVERAGE_COLORS = {
    numeric: 'border-green-200 bg-green-50',
    symbolic: 'border-yellow-200 bg-yellow-50',
    indirect: 'border-slate-200 bg-slate-50',
    unsupported: 'border-red-200 bg-red-50',
}

export function CostNode({ resource, isExpanded, isSelected, onToggle, onClick }: Props) {
    const icon = COVERAGE_ICONS[resource.coverageType]
    const borderColor = isSelected ? 'border-primary-500 bg-primary-50' : COVERAGE_COLORS[resource.coverageType]

    // Demo cost components if not provided
    const components: CostComponent[] = resource.components || [
        { name: 'Compute', category: 'Instance Hours', cost: resource.cost * 0.7, unit: 'hours', quantity: 730, rate: resource.cost * 0.7 / 730, isSymbolic: false },
        { name: 'Storage', category: 'EBS', cost: resource.cost * 0.2, unit: 'GB-month', quantity: 100, rate: 0.10, isSymbolic: false },
        { name: 'Data Transfer', category: 'Network', cost: resource.cost * 0.1, unit: 'GB', quantity: 50, rate: 0.09, isSymbolic: resource.coverageType === 'symbolic' },
    ]

    return (
        <div className={`border-l-4 ${borderColor} transition-colors`}>
            {/* Resource header */}
            <div
                className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-slate-50"
                onClick={() => { onToggle(); onClick(); }}
            >
                <div className="flex items-center gap-3">
                    {/* Expand icon */}
                    <svg
                        className={`w-4 h-4 text-slate-400 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                    >
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                    </svg>

                    {/* Coverage icon */}
                    <span className="text-lg">{icon}</span>

                    {/* Address */}
                    <div>
                        <div className="font-mono text-sm text-slate-900">{resource.address}</div>
                        <div className="text-xs text-slate-500">{resource.type}</div>
                    </div>
                </div>

                {/* Cost and confidence */}
                <div className="text-right">
                    <div className="font-semibold text-slate-900">
                        ${resource.cost.toFixed(2)}
                    </div>
                    <div className="text-xs text-slate-500">
                        {(resource.confidence * 100).toFixed(0)}% confidence
                    </div>
                </div>
            </div>

            {/* Expanded content */}
            {isExpanded && (
                <div className="px-4 pb-4 ml-8 border-l-2 border-slate-200">
                    {components.map((comp, i) => (
                        <div key={i} className="py-2 flex items-start">
                            {/* Tree line */}
                            <div className="flex items-center mr-3 text-slate-300">
                                {i === components.length - 1 ? '└──' : '├──'}
                            </div>

                            {/* Component */}
                            <div className="flex-1">
                                <div className="flex items-center gap-2">
                                    <span className={`font-medium ${comp.isSymbolic ? 'text-yellow-700' : 'text-slate-700'}`}>
                                        {comp.name}
                                    </span>
                                    <span className="text-xs text-slate-400">({comp.category})</span>
                                    {comp.isSymbolic && (
                                        <span className="text-xs px-1.5 py-0.5 bg-yellow-100 text-yellow-700 rounded">
                                            Symbolic
                                        </span>
                                    )}
                                </div>

                                <div className="mt-1 text-xs text-slate-500 space-y-0.5">
                                    <div>
                                        Rate: <span className="font-mono">${comp.rate.toFixed(4)}/{comp.unit}</span>
                                    </div>
                                    <div>
                                        Quantity: <span className="font-mono">{comp.quantity.toLocaleString()} {comp.unit}</span>
                                    </div>
                                    <div className="font-medium text-slate-700">
                                        Cost: <span className="font-mono">${comp.cost.toFixed(2)}</span>
                                    </div>
                                </div>

                                {comp.isSymbolic && comp.symbolicReason && (
                                    <div className="mt-2 text-xs text-yellow-700 bg-yellow-50 px-2 py-1 rounded">
                                        ⚠ {comp.symbolicReason}
                                    </div>
                                )}
                            </div>
                        </div>
                    ))}

                    {/* Lineage info */}
                    {resource.lineage && (
                        <div className="mt-3 pt-3 border-t border-slate-200 text-xs text-slate-500">
                            <div>Snapshot: <span className="font-mono">{resource.lineage.snapshotId}</span></div>
                            <div>Source: {resource.lineage.source}</div>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
