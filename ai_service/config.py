import os
from dotenv import load_dotenv

load_dotenv()

class Config:
    SECRET_KEY = os.getenv('SECRET_KEY', 'dev-secret-key')
    PORT = int(os.getenv('PORT', 5000))
    DEBUG = os.getenv('DEBUG', 'True').lower() == 'true'
    
    DISEASE_DATABASE_PATH = os.getenv('DISEASE_DB_PATH', 'disease_database.json')
    MODEL_PATH = os.getenv('MODEL_PATH', None)
    
    MAX_IMAGE_SIZE = 10 * 1024 * 1024
    ALLOWED_EXTENSIONS = {'png', 'jpg', 'jpeg', 'gif', 'bmp'}
    
    CONFIDENCE_THRESHOLD = 0.7
