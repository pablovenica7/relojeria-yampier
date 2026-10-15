// SweetAlert2 en blanco y negro, coherente con el resto del sitio.
// Se carga recién la primera vez que hace falta mostrar un aviso: así no
// viaja en el JS inicial de cada visitante (~20 KB gzip). La API es la misma
// de siempre: `swal.fire(opciones)` devuelve una promesa con el resultado.
let instance

function load() {
  instance ??= import('sweetalert2').then(({ default: Swal }) =>
    Swal.mixin({
      background: 'transparent',
      color: '#ffffff',
      customClass: {
        popup: 'yampier-swal',
        confirmButton: 'yampier-swal-confirm',
        cancelButton: 'yampier-swal-cancel',
      },
      buttonsStyling: false,
    })
  ).catch(err => {
    instance = undefined // sin conexión: el próximo aviso reintenta la descarga
    throw err
  })
  return instance
}

const swal = {
  fire: (...args) => load().then(s => s.fire(...args)),
}

export default swal
