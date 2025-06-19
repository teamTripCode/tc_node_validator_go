"""
Popup para visualizar los logs de una instancia del cluster
"""

import sys
import platform
from typing import Dict, Any, Optional, Union

from kivy.uix.popup import Popup
from kivy.uix.boxlayout import BoxLayout
from kivy.uix.scrollview import ScrollView
from kivy.uix.textinput import TextInput
from kivy.uix.button import Button  # Ensure this is Kivy's Button
from kivy.uix.widget import Widget
from kivy.clock import Clock, ClockEvent

from config.settings import COLORS
from core.utils import safe_read_process_output


def get_monospace_font() -> str:
    """Obtener fuente monoespaciada según el sistema operativo"""
    system = platform.system().lower()
    
    font_mapping = {
        'windows': 'Consolas',
        'darwin': 'Monaco',    # macOS
        'linux': 'Liberation Mono'
    }
    
    # Obtener fuente por sistema operativo, fallback a None
    return font_mapping.get(system, 'DejaVu Sans Mono')


def get_os_appropriate_font_size() -> str:
    """Obtener tamaño de fuente apropiado según el OS"""
    system = platform.system().lower()
    
    size_mapping = {
        'windows': '11sp',
        'darwin': '12sp',      # macOS tiende a necesitar tamaños ligeramente mayores
        'linux': '11sp'
    }
    
    return size_mapping.get(system, '11sp')


class LogViewerPopup(Popup):
    """Popup para visualizar logs de instancias del cluster"""
    
    def __init__(self, instance_info: Dict[str, Any], **kwargs):
        super().__init__(**kwargs)
        self.instance_info = instance_info
        self.refresh_event: Optional[ClockEvent] = None
        self.log_display: Optional[TextInput] = None
        self.auto_refresh_btn: Optional[Button] = None
        
        # Configurar popup
        self.title = f"Logs - {instance_info['type']} (Puerto {instance_info['port']})"
        self.size_hint = (0.9, 0.9)
        
        # Construir interfaz
        self._build_interface()
        
        # Configurar auto-refresh
        self.refresh_logs()
        self._start_auto_refresh()
    
    def _build_interface(self) -> None:
        """Construir la interfaz del popup"""
        # Layout principal
        content = BoxLayout(orientation='vertical', padding=10, spacing=10)
        
        # Área de logs con scroll
        self._setup_log_display()
        scroll = ScrollView()
        if self.log_display:
            scroll.add_widget(self.log_display)
        content.add_widget(scroll)
        
        # Área de botones
        button_layout = self._create_button_layout()
        content.add_widget(button_layout)
        
        # Asignar contenido
        self.content = content
    
    def _setup_log_display(self) -> None:
        """Configurar el área de visualización de logs"""
        try:
            font_name = get_monospace_font()
            font_size = get_os_appropriate_font_size()
            
            self.log_display = TextInput(
                multiline=True,
                readonly=True,
                font_size=font_size,
                background_color=COLORS.get('log_background', [0.1, 0.1, 0.1, 1]),
                foreground_color=COLORS.get('log_text', [0.9, 0.9, 0.9, 1]),
                font_name=font_name if font_name else None,
                cursor_color=[0, 0, 0, 0],  # Ocultar cursor en readonly
                selection_color=[0.3, 0.3, 0.3, 0.5]  # Color de selección sutil
            )
        except Exception as e:
            # Fallback si hay problemas con la configuración
            print(f"Warning: Error configurando log display: {e}")
            self.log_display = TextInput(
                multiline=True,
                readonly=True,
                font_size='11sp'
            )
    
    def _create_button_layout(self) -> BoxLayout:
        """Crear layout de botones"""
        button_layout = BoxLayout(size_hint_y=None, height=50, spacing=10)
        
        # Botón actualizar
        refresh_btn = Button(
            text="Actualizar",
            size_hint_x=0.3,
            background_color=COLORS.get('button_success', [0.2, 0.8, 0.2, 1])
        )
        # Uso de bind de Kivy
        refresh_btn.bind(on_press=self._on_refresh_pressed)
        
        # Botón auto-refresh toggle
        self.auto_refresh_btn = Button(
            text="Auto: ON",
            size_hint_x=0.3,
            background_color=COLORS.get('button_info', [0.2, 0.6, 0.8, 1])
        )
        self.auto_refresh_btn.bind(on_press=self._toggle_auto_refresh)
        
        # Botón cerrar
        close_btn = Button(
            text="Cerrar",
            size_hint_x=0.3,
            background_color=COLORS.get('button_danger', [0.8, 0.2, 0.2, 1])
        )
        close_btn.bind(on_press=self._on_close_pressed)
        
        # Agregar botones al layout
        button_layout.add_widget(refresh_btn)
        button_layout.add_widget(self.auto_refresh_btn)
        button_layout.add_widget(close_btn)
        
        return button_layout
    
    def _start_auto_refresh(self) -> None:
        """Iniciar el auto-refresh"""
        try:
            self.refresh_event = Clock.schedule_interval(
                lambda dt: self.refresh_logs(), 
                5  # Actualizar cada 5 segundos
            )
        except Exception as e:
            print(f"Warning: Error iniciando auto-refresh: {e}")
            self.refresh_event = None
    
    def _stop_auto_refresh(self) -> None:
        """Detener el auto-refresh de forma segura"""
        if self.refresh_event is not None:
            try:
                # Clock.unschedule es el método correcto para cancelar eventos
                Clock.unschedule(self.refresh_event)
                self.refresh_event = None
            except Exception as e:
                print(f"Warning: Error deteniendo auto-refresh: {e}")
                self.refresh_event = None
    
    def _on_refresh_pressed(self, button_instance: Button) -> None:
        """Callback para botón de actualización manual"""
        self.refresh_logs()
    
    def _on_close_pressed(self, button_instance: Button) -> None:
        """Callback para botón de cerrar"""
        self.dismiss()
    
    def _toggle_auto_refresh(self, button_instance: Button) -> None:
        """Toggle del auto-refresh"""
        if self.refresh_event is not None:
            # Desactivar auto-refresh
            self._stop_auto_refresh()
            if self.auto_refresh_btn:
                self.auto_refresh_btn.text = "Auto: OFF"
                self.auto_refresh_btn.background_color = COLORS.get('button_warning', [0.8, 0.6, 0.2, 1])
        else:
                if self.auto_refresh_btn:
                    self.auto_refresh_btn.text = "Auto: ON"
                    self.auto_refresh_btn.background_color = COLORS.get('button_info', [0.2, 0.6, 0.8, 1])
    
    def refresh_logs(self, *args) -> None:
        """Actualizar el contenido de los logs"""
        if not self.log_display:
            return
            
        try:
            # Obtener output del proceso
            output = safe_read_process_output(self.instance_info['process'])
            
            # Formatear información básica
            basic_info = self._format_basic_info()
            
            # Combinar toda la información
            full_logs = self._format_full_logs(basic_info, output)
            
            # Actualizar display
            self.log_display.text = full_logs
            
            # Auto-scroll al final si hay contenido nuevo
            self._auto_scroll_to_bottom()
            
        except Exception as e:
            error_msg = f"Error actualizando logs: {e}\n"
            error_msg += f"Instancia: {self.instance_info.get('type', 'Unknown')}\n"
            error_msg += f"Puerto: {self.instance_info.get('port', 'Unknown')}"
            self.log_display.text = error_msg
    
    def _format_basic_info(self) -> str:
        """Formatear información básica de la instancia"""
        process = self.instance_info.get('process')
        pid = 'N/A'
        status = 'Desconocido'
        
        if process:
            try:
                pid = str(process.pid) if hasattr(process, 'pid') else 'N/A'
                if hasattr(process, 'poll'):
                    poll_result = process.poll()
                    status = 'Ejecutándose' if poll_result is None else f'Terminado (código: {poll_result})'
            except Exception as e:
                status = f'Error obteniendo estado: {e}'
        
        return (
            f"=== INFORMACIÓN DE LA INSTANCIA ===\n"
            f"Tipo: {self.instance_info.get('type', 'N/A')}\n"
            f"Puerto: {self.instance_info.get('port', 'N/A')}\n"
            f"Directorio: {self.instance_info.get('datadir', 'N/A')}\n"
            f"PID: {pid}\n"
            f"Estado: {status}\n"
            f"Plataforma: {platform.system()} {platform.release()}\n"
            "=" * 50 + "\n\n"
        )
    
    def _format_full_logs(self, basic_info: str, output: Dict[str, Any]) -> str:
        """Formatear logs completos"""
        full_logs = basic_info
        
        # Agregar STDOUT si existe
        if output.get('stdout'):
            full_logs += "=== SALIDA ESTÁNDAR (STDOUT) ===\n"
            full_logs += output['stdout']
            full_logs += "\n" + "=" * 40 + "\n\n"
        
        # Agregar STDERR si existe
        if output.get('stderr'):
            full_logs += "=== ERRORES (STDERR) ===\n"
            full_logs += output['stderr']
            full_logs += "\n" + "=" * 40 + "\n\n"
        
        # Mensaje si no hay logs
        if not output.get('stdout') and not output.get('stderr'):
            full_logs += "📝 No hay logs disponibles.\n"
            full_logs += "El proceso puede estar:\n"
            full_logs += "  • Iniciándose aún\n"
            full_logs += "  • Ejecutándose sin generar salida\n"
            full_logs += "  • Configurado para no mostrar logs verbosos\n\n"
            full_logs += "💡 Intenta actualizar en unos segundos o verificar la configuración de logs."
        
        return full_logs
    
    def _auto_scroll_to_bottom(self) -> None:
        """Auto-scroll al final del texto si es apropiado"""
        try:
            if self.log_display:
                # En Kivy, usar Clock.schedule_once para el scroll después del próximo frame
                Clock.schedule_once(self._do_scroll, 0)
        except Exception:
            # Ignorar errores de scroll
            pass
    
    def _do_scroll(self, dt: float) -> None:
        """Realizar el scroll efectivo"""
        try:
            if self.log_display:
                # Mover el cursor al final
                self.log_display.cursor = (len(self.log_display.text), 0)
        except Exception:
            pass
    
    def dismiss(self, *args) -> None:
        """Cerrar el popup y limpiar recursos"""
        # Cancelar auto-refresh si está activo
        self._stop_auto_refresh()
        
        # Llamar al dismiss del padre
        super().dismiss(*args)
    
    def on_dismiss(self) -> None:
        """Callback cuando el popup se cierra"""
        # Asegurar limpieza de recursos
        self._stop_auto_refresh()


# Función de utilidad para crear el popup fácilmente
def show_instance_logs(instance_info: Dict[str, Any]) -> LogViewerPopup:
    """
    Función de conveniencia para mostrar logs de una instancia
    
    Args:
        instance_info: Diccionario con información de la instancia
        
    Returns:
        LogViewerPopup: La instancia del popup creado
    """
    popup = LogViewerPopup(instance_info)
    popup.open()
    return popup