import type { ResourceChange } from '../../types'

interface Props {
    change: ResourceChange
}

export function DiffNarrative({ change }: Props) {
    const deltaColor = change.delta >= 0 ? 'text-red-600' : 'text-green-600'
    const deltaSign = change.delta >= 0 ? '+' : ''

    return (
        <div className="bg-white rounded-lg border border-slate-200 p-4">
            {/* Header */}
            <div className="flex items-center justify-between mb-4">
                <div>
                    <div className="font-mono text-sm text-slate-900">{change.address}</div>
                    <div className="text-xs text-slate-500">{change.type}</div>
                </div>
                <div className={`text-xl font-bold ${deltaColor}`}>
                    {deltaSign}${Math.abs(change.delta).toFixed(2)}
                </div>
            </div>

            {/* Before → After */}
            <div className="flex items-center gap-4 mb-4 text-sm">
                <div className="flex items-center gap-2">
                    <span className="text-slate-500">Before:</span>
                    <span className="font-semibold">${change.oldCost.toFixed(2)}</span>
                </div>
                <span className="text-slate-400">→</span>
                <div className="flex items-center gap-2">
                    <span className="text-slate-500">After:</span>
                    <span className="font-semibold">${change.newCost.toFixed(2)}</span>
                </div>
            </div>

            {/* Drivers */}
            {change.drivers.length > 0 && (
                <div className="mb-4">
                    <div className="text-xs font-medium text-slate-500 uppercase mb-2">Change Drivers</div>
                    <div className="space-y-2">
                        {change.drivers.map((driver, i) => (
                            <div key={i} className="flex items-center justify-between text-sm p-2 bg-slate-50 rounded">
                                <div>
                                    <span className="text-slate-700">{driver.attribute}:</span>
                                    <span className="ml-2 text-red-600 line-through">{driver.oldValue}</span>
                                    <span className="ml-1">→</span>
                                    <span className="ml-1 text-green-600">{driver.newValue}</span>
                                </div>
                                <div className={`font-mono text-xs ${driver.impact >= 0 ? 'text-red-600' : 'text-green-600'}`}>
                                    {driver.impact >= 0 ? '+' : ''}${driver.impact.toFixed(2)}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Narrative */}
            <div className="p-3 bg-slate-50 rounded-lg text-sm text-slate-700">
                <div className="font-medium text-slate-900 mb-1">Explanation</div>
                {change.narrative}
            </div>
        </div>
    )
}
