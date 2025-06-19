"""
Estilos y temas para la interfaz de usuario
"""

from kivy.core.window import Window
from config.settings import COLORS, APP_CONFIG

def apply_dark_theme():
    """Aplicar tema oscuro a la ventana principal"""
    Window.clearcolor = APP_CONFIG['window_clearcolor']

def get_button_style(button_type: str = 'default') -> dict:
    """Obtener estilos para botones"""
    styles = {
        'default': {
            'background_color': (0.3, 0.3, 0.3, 1)
        },
        'success': {
            'background_color': COLORS['button_success']
        },
        'danger': {
            'background_color': COLORS['button_danger']
        },
        'info': {
            'background_color': COLORS['info']
        }
    }
    return styles.get(button_type, styles['default'])

def get_text_style(text_type: str = 'primary') -> dict:
    """Obtener estilos para texto"""
    styles = {
        'primary': {
            'color': COLORS['text_primary']
        },
        'secondary': {
            'color': COLORS['text_secondary']
        },
        'muted': {
            'color': COLORS['text_muted']
        }
    }
    return styles.get(text_type, styles['primary'])

def get_widget_background_color() -> tuple:
    """Obtener color de fondo para widgets"""
    return COLORS['widget_background']