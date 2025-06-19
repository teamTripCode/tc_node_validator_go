"""
Configuración global de la aplicación
"""

import os
import sys
import subprocess

# Configuración de la aplicación
APP_CONFIG = {
    'title': 'Monitor de Cluster Go',
    'window_size': (1200, 800),
    'window_clearcolor': (0.1, 0.1, 0.1, 1),
    'font_size': {
        'title': '20sp',
        'header': '16sp',
        'normal': '14sp',
        'small': '12sp',
        'tiny': '11sp'
    }
}

# Configuración del cluster
CLUSTER_CONFIG = {
    'ports': [3001, 3002, 3003, 3004, 3005, 3006, 3007, 3008, 3009, 
              3010, 3011, 3012, 3013, 3014, 3015, 3016, 3017, 3018, 
              3019, 3020, 3021, 3022, 3023],
    'seed_node': 'localhost:3000',
    'data_base_dir': '../../../data',
    'principal_dir': '../../../data/principal',
    'delegate_prefix': '../../../data/delegate-',
    'startup_delay': 3,  # segundos entre inicio del principal y delegados
    'instance_delay': 0.2  # segundos entre inicio de cada delegado
}

# Configuración de monitoreo
MONITOR_CONFIG = {
    'update_interval': 60,     # segundos - actualización completa
    'quick_update_interval': 10,  # segundos - actualización rápida
    'log_refresh_interval': 5,    # segundos - actualización de logs
    'max_log_lines': 1000        # máximo de líneas en logs
}

# Configuración de colores (tema oscuro)
COLORS = {
    'background': (0.1, 0.1, 0.1, 1),
    'widget_background': (0.15, 0.15, 0.15, 1),
    'text_primary': (1, 1, 1, 1),
    'text_secondary': (0.8, 0.8, 0.8, 1),
    'text_muted': (0.6, 0.6, 0.6, 1),
    'success': (0.2, 0.8, 0.2, 1),
    'warning': (0.8, 0.8, 0.2, 1),
    'error': (0.8, 0.2, 0.2, 1),
    'info': (0.3, 0.5, 0.8, 1),
    'button_success': (0.2, 0.6, 0.2, 1),
    'button_danger': (0.6, 0.2, 0.2, 1),
    'log_background': (0.05, 0.05, 0.05, 1),
    'log_text': (0.9, 0.9, 0.9, 1)
}

# Configuración de fuentes
FONT_CONFIG = {
    'monospace_fonts': [
        'Consolas',        # Windows
        'Courier New',     # Windows/Cross-platform
        'Monaco',          # macOS
        'DejaVu Sans Mono', # Linux
        'Liberation Mono', # Linux
        'Menlo',          # macOS
        'Lucida Console'  # Windows
    ]
}

# Configuración de logging
LOGGING_CONFIG = {
    'level': 'INFO',
    'format': '%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    'file': 'cluster_monitor.log',
    'max_size': 10 * 1024 * 1024,  # 10MB
    'backup_count': 3
}

# Status icons
STATUS_ICONS = {
    'active': '🟢',
    'starting': '🟡',
    'inactive': '🔴',
    'error': '❌',
    'stopped': '⏹️',
    'warning': '⚠️'
}

# Mensajes de estado
STATUS_MESSAGES = {
    'cluster_stopped': f"{STATUS_ICONS['inactive']} Cluster detenido",
    'cluster_starting': f"{STATUS_ICONS['starting']} Iniciando cluster...",
    'cluster_active': f"{STATUS_ICONS['active']} Cluster activo",
    'cluster_stopping': f"{STATUS_ICONS['starting']} Deteniendo cluster...",
    'cluster_error': f"{STATUS_ICONS['error']} Error en cluster",
    'instance_active': f"{STATUS_ICONS['active']} ACTIVO",
    'instance_starting': f"{STATUS_ICONS['starting']} INICIANDO",
    'instance_inactive': f"{STATUS_ICONS['inactive']} INACTIVO"
}


# Handlers

# Configuración específica del sistema operativo

def get_os_config():
    """Obtener configuración específica del OS"""
    if os.name == 'nt':  # Windows
        return {
            'preferred_font': 'Consolas',
            'process_creation_flags': subprocess.CREATE_NO_WINDOW,
            'signal_handling': 'windows',
            'path_separator': '\\',
            'line_ending': '\r\n',
            'shell_command': 'cmd',
            'supports_killpg': False,
            'supports_setsid': False
        }
    else:  # Unix-like (Linux, macOS)
        return {
            'preferred_font': 'Liberation Mono',
            'process_creation_flags': None,
            'signal_handling': 'unix',
            'path_separator': '/',
            'line_ending': '\n',
            'shell_command': 'bash',
            'supports_killpg': hasattr(os, 'killpg'),
            'supports_setsid': hasattr(os, 'setsid')
        }

def get_platform_info():
    """Obtener información detallada de la plataforma"""
    platform_data = {
        'os_name': os.name,
        'platform': sys.platform,
        'is_windows': sys.platform.startswith('win'),
        'is_posix': hasattr(os, 'fork'),
        'python_version': sys.version,
        'architecture': sys.maxsize > 2**32 and '64-bit' or '32-bit'
    }
    
    if platform_data['is_windows']:
        try:
            version = sys.getwindowsversion()
            platform_data['os_version'] = f"Windows {version.major}.{version.minor}.{version.build}"
            platform_data['os_build'] = version.build
        except AttributeError:
            platform_data['os_version'] = "Windows (versión desconocida)"
    else:
        try:
            import platform
            platform_data['os_version'] = platform.platform()
            platform_data['kernel_version'] = platform.release()
        except ImportError:
            platform_data['os_version'] = f"{sys.platform} (detalles no disponibles)"
    
    return platform_data

def validate_os_compatibility():
    """Validar que el sistema operativo sea compatible"""
    config = get_os_config()
    platform_info = get_platform_info()
    
    compatibility_report = {
        'is_compatible': True,
        'warnings': [],
        'features': {
            'process_groups': config['supports_killpg'],
            'session_control': config['supports_setsid'],
            'background_processes': True,  # Soportado en ambos
            'signal_handling': config['signal_handling'] in ['windows', 'unix']
        }
    }
    
    # Verificaciones específicas
    if not platform_info['is_windows'] and not platform_info['is_posix']:
        compatibility_report['warnings'].append(
            "Sistema operativo no completamente soportado - funcionalidad limitada"
        )
    
    if platform_info['is_windows'] and not config['supports_killpg']:
        compatibility_report['warnings'].append(
            "Windows no soporta killpg - usando métodos alternativos"
        )
    
    # Verificar Python version (debe ser 3.7+)
    python_version = sys.version_info
    if python_version.major < 3 or (python_version.major == 3 and python_version.minor < 7):
        compatibility_report['is_compatible'] = False
        compatibility_report['warnings'].append(
            f"Python {python_version.major}.{python_version.minor} no soportado - requiere Python 3.7+"
        )
    
    return compatibility_report

# Ejemplo de uso y testing

    print("=== Configuración del Sistema Operativo ===")
    
    config = get_os_config()
    print(f"Configuración OS: {config}")
    
    platform_info = get_platform_info()
    print(f"\nInformación de plataforma: {platform_info}")
    
    compatibility = validate_os_compatibility()
    print(f"\nCompatibilidad: {compatibility}")
    
    if compatibility['warnings']:
        print("\nAdvertencias:")
        for warning in compatibility['warnings']:
            print(f"  - {warning}")