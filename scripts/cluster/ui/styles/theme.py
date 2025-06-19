"""
Configuración de tema y colores para la aplicación
"""

from kivy.graphics import Color, Rectangle
from config.settings import COLORS, STATUS_ICONS

class ThemeManager:
    """Gestor de temas para la aplicación"""
    
    @staticmethod
    def apply_widget_background(widget, color_key: str = 'widget_background'):
        """Aplicar fondo a un widget"""
        with widget.canvas.before:
            Color(*COLORS[color_key])
            widget.rect = Rectangle(pos=widget.pos, size=widget.size)
        widget.bind(pos=ThemeManager._update_rect, size=ThemeManager._update_rect)
    
    @staticmethod
    def _update_rect(widget, *args):
        """Actualizar rectángulo de fondo"""
        if hasattr(widget, 'rect'):
            widget.rect.pos = widget.pos
            widget.rect.size = widget.size
    
    @staticmethod
    def get_status_color(status: str) -> tuple:
        """Obtener color según el estado"""
        status_colors = {
            'active': COLORS['success'],
            'starting': COLORS['warning'],
            'inactive': COLORS['error'],
            'error': COLORS['error']
        }
        return status_colors.get(status, COLORS['text_muted'])
    
    @staticmethod
    def get_status_icon(status: str) -> str:
        """Obtener icono según el estado"""
        return STATUS_ICONS.get(status, STATUS_ICONS['inactive'])
    
    @staticmethod
    def create_status_text(status: str, text: str = "") -> str:
        """Crear texto con icono de estado"""
        icon = ThemeManager.get_status_icon(status)
        return f"{icon} {text}".strip()