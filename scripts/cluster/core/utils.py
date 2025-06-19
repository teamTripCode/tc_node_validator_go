"""
Utilidades generales para el monitor de cluster
"""

import os
import psutil
import logging
from typing import Optional, List, Dict, Any
from config.settings import FONT_CONFIG, get_os_config

logger = logging.getLogger(__name__)

def get_monospace_font() -> Optional[str]:
    """Obtener una fuente monoespaciada disponible en el sistema"""
    os_config = get_os_config()
    preferred_font = os_config.get('preferred_font')
    
    # Intentar con la fuente preferida del sistema
    if preferred_font:
        try:
            return preferred_font
        except Exception:
            logger.debug(f"Fuente preferida {preferred_font} no disponible")
    
    # Probar otras fuentes
    for font in FONT_CONFIG['monospace_fonts']:
        try:
            # En un entorno real, aquí verificaríamos si la fuente existe
            # Por simplicidad, devolvemos la primera disponible
            return font
        except Exception:
            continue
    
    logger.warning("No se encontró fuente monoespaciada, usando predeterminada")
    return None

def is_port_in_use(port: int) -> bool:
    """Verificar si un puerto específico está en uso"""
    try:
        for conn in psutil.net_connections():
            if hasattr(conn, 'laddr') and conn.laddr and conn.laddr.port == port:
                if conn.status == 'LISTEN':
                    return True
        return False
    except Exception as e:
        logger.error(f"Error verificando puerto {port}: {e}")
        return False

def get_process_info(pid: int) -> Optional[Dict[str, Any]]:
    """Obtener información detallada de un proceso"""
    try:
        process = psutil.Process(pid)
        return {
            'pid': process.pid,
            'name': process.name(),
            'status': process.status(),
            'cpu_percent': process.cpu_percent(),
            'memory_info': process.memory_info(),
            'create_time': process.create_time(),
            'cmdline': process.cmdline()
        }
    except (psutil.NoSuchProcess, psutil.AccessDenied) as e:
        logger.error(f"Error obteniendo info del proceso {pid}: {e}")
        return None

def format_bytes(bytes_value: float) -> str:
    """Formatear bytes en unidades legibles"""
    for unit in ['B', 'KB', 'MB', 'GB']:
        if bytes_value < 1024.0:
            return f"{int(bytes_value):.1f} {unit}"
        bytes_value = float(bytes_value) / 1024.0
    return f"{int(bytes_value):.1f} TB"

def format_uptime(seconds: float) -> str:
    """Formatear tiempo de actividad en formato legible"""
    if seconds < 60:
        return f"{int(seconds)}s"
    elif seconds < 3600:
        minutes = int(seconds // 60)
        secs = int(seconds % 60)
        return f"{minutes}m {secs}s"
    else:
        hours = int(seconds // 3600)
        minutes = int((seconds % 3600) // 60)
        return f"{hours}h {minutes}m"

def safe_read_process_output(process, max_lines: int = 100) -> Dict[str, str]:
    """Leer salida de proceso de forma segura"""
    stdout_data = ""
    stderr_data = ""
    
    try:
        if process and process.poll() is None:
            # Leer stdout
            if hasattr(process, 'stdout') and process.stdout:
                try:
                    if hasattr(process.stdout, 'readable') and process.stdout.readable():
                        raw_data = process.stdout.read()
                        if raw_data:
                            stdout_data = raw_data.decode('utf-8', errors='ignore')
                    else:
                        stdout_data = "Logs no disponibles (proceso activo)"
                except Exception as e:
                    stdout_data = f"Error leyendo stdout: {str(e)}"
            
            # Leer stderr
            if hasattr(process, 'stderr') and process.stderr:
                try:
                    if hasattr(process.stderr, 'readable') and process.stderr.readable():
                        raw_data = process.stderr.read()
                        if raw_data:
                            stderr_data = raw_data.decode('utf-8', errors='ignore')
                    else:
                        stderr_data = "Error logs no disponibles (proceso activo)"
                except Exception as e:
                    stderr_data = f"Error leyendo stderr: {str(e)}"
        else:
            stdout_data = "Proceso no activo"
            stderr_data = "Proceso no activo"
    
    except Exception as e:
        stdout_data = f"Error accediendo al proceso: {str(e)}"
        stderr_data = f"Error accediendo al proceso: {str(e)}"
    
    # Limitar número de líneas
    if stdout_data:
        lines = stdout_data.split('\n')
        if len(lines) > max_lines:
            stdout_data = '\n'.join(lines[-max_lines:])
            stdout_data = f"... (mostrando últimas {max_lines} líneas)\n" + stdout_data
    
    if stderr_data:
        lines = stderr_data.split('\n')
        if len(lines) > max_lines:
            stderr_data = '\n'.join(lines[-max_lines:])
            stderr_data = f"... (mostrando últimas {max_lines} líneas)\n" + stderr_data
    
    return {
        'stdout': stdout_data,
        'stderr': stderr_data
    }

def create_directory_if_not_exists(directory: str) -> bool:
    """Crear directorio si no existe"""
    try:
        if not os.path.exists(directory):
            os.makedirs(directory, exist_ok=True)
            logger.info(f"Directorio creado: {directory}")
        return True
    except Exception as e:
        logger.error(f"Error creando directorio {directory}: {e}")
        return False

def get_available_ports(start_port: int = 3000, count: int = 25) -> List[int]:
    """Obtener lista de puertos disponibles"""
    available_ports = []
    current_port = start_port
    
    while len(available_ports) < count:
        if not is_port_in_use(current_port):
            available_ports.append(current_port)
        current_port += 1
        
        # Evitar bucle infinito
        if current_port > start_port + 1000:
            break
    
    return available_ports

def validate_go_environment() -> Dict[str, Any]:
    """Validar que el entorno Go esté configurado correctamente"""
    result = {
        'go_available': False,
        'go_version': None,
        'go_path': None,
        'main_go_exists': False,
        'error': None
    }
    
    try:
        # Verificar si go está disponible
        import subprocess
        process = subprocess.run(['go', 'version'], 
                               capture_output=True, text=True, timeout=10)
        
        if process.returncode == 0:
            result['go_available'] = True
            result['go_version'] = process.stdout.strip()
        
        # Verificar GOPATH
        process = subprocess.run(['go', 'env', 'GOPATH'], 
                               capture_output=True, text=True, timeout=10)
        if process.returncode == 0:
            result['go_path'] = process.stdout.strip()
        
        # Verificar si main.go existe
        if os.path.exists('main.go'):
            result['main_go_exists'] = True
        
    except subprocess.TimeoutExpired:
        result['error'] = "Timeout verificando Go"
    except FileNotFoundError:
        result['error'] = "Go no está instalado o no está en PATH"
    except Exception as e:
        result['error'] = f"Error verificando Go: {str(e)}"
    
    return result

def cleanup_temp_files(directory: str = "data") -> None:
    """Limpiar archivos temporales"""
    try:
        if os.path.exists(directory):
            for root, dirs, files in os.walk(directory):
                for file in files:
                    if file.endswith(('.tmp', '.temp', '.log')):
                        file_path = os.path.join(root, file)
                        try:
                            os.remove(file_path)
                            logger.debug(f"Archivo temporal eliminado: {file_path}")
                        except Exception as e:
                            logger.warning(f"No se pudo eliminar {file_path}: {e}")
    except Exception as e:
        logger.error(f"Error limpiando archivos temporales: {e}")