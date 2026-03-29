declare module '@examples/sample-composition.json' {
  const value: {
    composition: {
      blocks: Array<{
        kind: string
        name: string
        parameters?: Record<string, string>
        inputs?: Record<string, string>
      }>
    }
  }
  export default value
}
