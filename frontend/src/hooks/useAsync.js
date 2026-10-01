import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * useAsync: reemplaza el patrón repetido de
 *   const [items, setItems] = useState([])
 *   const [state, setState] = useState('loading')
 *   useEffect(() => { ...fetch... }, [deps])
 * que aparecía igual en Relojes, RelojDetalle, WatchesList e Inquiries.
 *
 * `state` es uno de: 'loading' | 'success' | 'error'. El caso "vacío" (lista
 * sin resultados) se decide en el componente según `data`, porque depende de
 * cada pantalla (no es responsabilidad de este hook).
 *
 * No es una librería de fetching (no cachea entre componentes ni entre
 * pantallas): es deliberadamente simple, para no traer una dependencia grande
 * cuando el problema real es solo evitar repetir el mismo boilerplate.
 */
export function useAsync(asyncFn, deps = []) {
  const [state, setState] = useState('loading')
  const [data, setData] = useState(null)
  const [error, setError] = useState(null)
  const reloadToken = useRef(0)

  const run = useCallback(() => {
    const myToken = ++reloadToken.current
    setState('loading')
    setError(null)
    asyncFn()
      .then((result) => {
        if (myToken !== reloadToken.current) return // llegó una respuesta vieja: se descarta
        setData(result)
        setState('success')
      })
      .catch((err) => {
        if (myToken !== reloadToken.current) return
        setError(err)
        setState('error')
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)

  useEffect(() => { run() }, [run])

  return { state, data, error, reload: run, setData }
}
