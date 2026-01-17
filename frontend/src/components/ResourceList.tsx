import { useState } from 'react'
import type { ResourceCost } from '../types'

interface Props {
    resources: ResourceCost[]
    symbolicReasons: Record<string, string[]>
}

const COVERAGE_BADGES = {
    numeric: { bg: 'bg-green-100', text: 'text-green-800', label: 'Numeric' },
    symbolic: { bg: 'bg-purple-100', text: 'text-purple-800', label: 'Symbolic' },
    indirect: { bg: 'bg-gray-100', text: 'text-gray-800', label: 'Indirect' },
    unsupported: { bg: 'bg-red-100', text: 'text-red-800', label: 'Unsupported' },
}

export function ResourceList({ resources, symbolicReasons }: Props) {
    const [expandedResource, setExpandedResource] = useState<string | null>(null)
    const [filter, setFilter] = useState<string>('all')

    const filteredResources = filter === 'all'
        ? resources
        : resources.filter(r => r.coverageType === filter)

    return (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200">
            <div className="p-6 border-b border-slate-200 flex justify-between items-center">
                <h2 className="text-lg font-semibold text-slate-900">Resources ({resources.length})</h2>

                <div className="flex gap-2">
                    {['all', 'numeric', 'symbolic', 'indirect'].map((f) => (
                        <button
                            key={f}
                            onClick={() => setFilter(f)}
                            className={`px-3 py-1 text-sm rounded-full transition-colors ${filter === f
                                    ? 'bg-primary-600 text-white'
                                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                                }`}
                        >
                            {f.charAt(0).toUpperCase() + f.slice(1)}
                        </button>
                    ))}
                </div>
            </div>

            <div className="divide-y divide-slate-100">
                {filteredResources.map((resource) => {
                    const badge = COVERAGE_BADGES[resource.coverageType]
                    const isExpanded = expandedResource === resource.address
                    const reasons = symbolicReasons[resource.type] || []

                    return (
                        <div key={resource.address} className="p-4">
                            <div
                                className="flex items-center justify-between cursor-pointer"
                                onClick={() => setExpandedResource(isExpanded ? null : resource.address)}
                            >
                                <div className="flex items-center gap-3">
                                    <span className={`px-2 py-0.5 text-xs rounded-full ${badge.bg} ${badge.text}`}>
                                        {badge.label}
                                    </span>
                                    <span className="font-mono text-sm text-slate-900">{resource.address}</span>
                                    <span className="text-xs text-slate-500">{resource.type}</span>
                                </div>

                                <div className="flex items-center gap-4">
                                    <div className="text-right">
                                        <div className="font-semibold text-slate-900">
                                            ${resource.cost.toFixed(2)}
                                        </div>
                                        <div className="text-xs text-slate-500">
                                            Confidence: {(resource.confidence * 100).toFixed(0)}%
                                        </div>
                                    </div>

                                    <svg
                                        className={`w-5 h-5 text-slate-400 transition-transform ${isExpanded ? 'rotate-180' : ''}`}
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                    >
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                    </svg>
                                </div>
                            </div>

                            {isExpanded && (
                                <div className="mt-4 ml-8 p-4 bg-slate-50 rounded-lg">
                                    <div className="grid grid-cols-2 gap-4 text-sm">
                                        <div>
                                            <span className="text-slate-500">Coverage Type:</span>
                                            <span className="ml-2 font-medium">{resource.coverageType}</span>
                                        </div>
                                        <div>
                                            <span className="text-slate-500">Confidence:</span>
                                            <span className="ml-2 font-medium">{(resource.confidence * 100).toFixed(0)}%</span>
                                        </div>
                                    </div>

                                    {reasons.length > 0 && (
                                        <div className="mt-3 p-3 bg-purple-50 rounded border border-purple-100">
                                            <div className="text-sm font-medium text-purple-800 mb-2">
                                                Why is this symbolic?
                                            </div>
                                            <ul className="text-sm text-purple-700 space-y-1">
                                                {reasons.map((reason, i) => (
                                                    <li key={i}>• {reason}</li>
                                                ))}
                                            </ul>
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                    )
                })}
            </div>
        </div>
    )
}
