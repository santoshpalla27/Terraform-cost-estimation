interface Props {
    types: string[]
}

export function UnsupportedWarning({ types }: Props) {
    if (types.length === 0) return null

    return (
        <div className="bg-red-50 border border-red-200 rounded-xl p-6">
            <div className="flex items-start gap-4">
                <div className="w-10 h-10 bg-red-100 rounded-full flex items-center justify-center flex-shrink-0">
                    <svg className="w-5 h-5 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                </div>

                <div className="flex-1">
                    <h3 className="text-lg font-semibold text-red-900 mb-2">
                        Unsupported Resources Detected
                    </h3>
                    <p className="text-sm text-red-700 mb-4">
                        The following resource types are not yet supported for cost estimation.
                        Their costs are not included in the total and are marked as unsupported in coverage.
                    </p>

                    <div className="flex flex-wrap gap-2">
                        {types.map((type) => (
                            <span
                                key={type}
                                className="px-3 py-1.5 bg-red-100 text-red-800 rounded-lg text-sm font-mono"
                            >
                                {type}
                            </span>
                        ))}
                    </div>

                    <div className="mt-4 pt-4 border-t border-red-200 text-sm text-red-700">
                        <strong>Impact:</strong> Total cost may be underestimated. Coverage is reduced proportionally.
                    </div>
                </div>
            </div>
        </div>
    )
}
