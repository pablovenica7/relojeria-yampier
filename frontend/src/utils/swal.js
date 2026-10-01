import Swal from 'sweetalert2'

// Instancia de SweetAlert2 en blanco y negro, coherente con el resto del sitio.
const swal = Swal.mixin({
  background: 'transparent',
  color: '#ffffff',
  customClass: {
    popup: 'yampier-swal',
    confirmButton: 'yampier-swal-confirm',
    cancelButton: 'yampier-swal-cancel',
  },
  buttonsStyling: false,
})

export default swal
